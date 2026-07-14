// Package instaray implements a Telegram bot that parse Twitter, Instagram and
// TikTok embeds.
package instaray

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"

	"github.com/Madh93/instaray/internal/config"
	"github.com/Madh93/instaray/internal/embed"
	"github.com/Madh93/instaray/internal/logging"
)

// Config holds the configuration for the Instaray.
type Config struct {
	Telegram *config.TelegramConfig
}

// Instaray represents the bot with its dependencies, including the Telegram
// bot, logger and other options.
type Instaray struct {
	telegram  *Telegram
	logger    *logging.Logger
	allowlist []int64
	threads   []int
	embeds    []*embed.Embed
}

// New creates a new Instaray instance, initializing the Telegram client.
func New(logger *logging.Logger, config *Config) *Instaray {
	return &Instaray{
		telegram:  createTelegram(logger, config.Telegram),
		allowlist: config.Telegram.Allowlist,
		threads:   config.Telegram.Threads,
		logger:    logger,
		embeds: []*embed.Embed{
			embed.New("instagram", "mdbinstagram.com"),
			embed.New("twitter", "fxtwitter.com"),
			embed.New("x", "fixupx.com"),
			embed.New("tiktok", "vxtiktok.com"),
		},
	}
}

// Run starts the bot and handles incoming messages.
func (i *Instaray) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Set default handler
	i.telegram.RegisterHandlerMatchFunc(func(*TelegramUpdate) bool { return true }, i.handler)

	// Start the bot
	i.telegram.Start(ctx)

	return nil
}

// handler is the main handler for incoming messages. It processes the message
// and sends a response back to the user.
func (i Instaray) handler(ctx context.Context, _ *Bot, update *TelegramUpdate) {
	if update.Message == nil {
		return
	}

	msg := TelegramMessage(*update.Message)

	// Check if the chat ID is allowed
	if !i.isChatIdAllowed(msg.Chat.ID) {
		i.logger.Warn(fmt.Sprintf("Received message from not allowed chat ID. Allowed chats IDs: %v", i.allowlist), msg.Attrs()...)
		return
	}

	// Check if the thread ID is allowed
	if !i.isThreadIdAllowed(msg.MessageThreadID) {
		i.logger.Warn(fmt.Sprintf("Received message from not allowed thread ID. Allowed thread IDs: %v", i.threads), msg.Attrs()...)
		return
	}

	i.logger.Debug("Received message from allowed chat ID and allowed thread ID", msg.Attrs()...)

	// Parse the message to get the fixed URL
	if ok := i.parseMessage(&msg); !ok {
		return
	}

	// Send back new message with fixed URL
	i.logger.Debug("Sending updated message with fixed URL", msg.Attrs()...)
	if err := i.telegram.SendNewMessage(ctx, &msg); err != nil {
		i.logger.Error("Failed to send new message", msg.AttrsWithError(err)...)
		return
	}

	// Delete original message
	i.logger.Debug("Deleting original message", msg.Attrs()...)
	if err := i.telegram.DeleteOriginalMessage(ctx, &msg); err != nil {
		i.logger.Error("Failed to delete original message", msg.AttrsWithError(err)...)
		return
	}

	i.logger.Info("Updated message", msg.Attrs()...)
}

// isChatIdAllowed checks if the chat ID is allowed to receive messages.
func (i Instaray) isChatIdAllowed(chatId int64) bool {
	return len(i.allowlist) == 0 || slices.Contains(i.allowlist, chatId)
}

// isThreadIdAllowed checks if the thread ID is allowed to receive messages.
func (i Instaray) isThreadIdAllowed(threadId int) bool {
	return len(i.threads) == 0 || slices.Contains(i.threads, threadId)
}

// parseMessage parses the incoming Telegram message and returns the fixed URL.
func (i Instaray) parseMessage(msg *TelegramMessage) bool {
	// Ignore message with multiple words
	if strings.Contains(strings.TrimSpace(msg.Text), " ") {
		return false
	}

	// Fix embed
	for _, embed := range i.embeds {
		if embed.Check(msg.Text) {
			msg.Text = embed.Replace(msg.Text)
			return true
		}
	}

	return false
}
