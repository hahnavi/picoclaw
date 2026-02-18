package channels

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
	"github.com/sipeed/picoclaw/pkg/voice"
)

const (
	transcriptionTimeout = 30 * time.Second
	sendTimeout          = 10 * time.Second
)

type DiscordChannel struct {
	*BaseChannel
	session     *discordgo.Session
	config      config.DiscordConfig
	transcriber *voice.GroqTranscriber
	ctx         context.Context
	botUserID   string // Bot user ID for mention detection
}

func NewDiscordChannel(cfg config.DiscordConfig, bus *bus.MessageBus) (*DiscordChannel, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	base := NewBaseChannel("discord", cfg, bus, cfg.AllowFrom)

	return &DiscordChannel{
		BaseChannel: base,
		session:     session,
		config:      cfg,
		transcriber: nil,
		ctx:         context.Background(),
	}, nil
}

func (c *DiscordChannel) SetTranscriber(transcriber *voice.GroqTranscriber) {
	c.transcriber = transcriber
}

func (c *DiscordChannel) getContext() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *DiscordChannel) Start(ctx context.Context) error {
	logger.InfoC("discord", "Starting Discord bot")

	c.ctx = ctx
	c.session.AddHandler(c.handleMessage)

	if err := c.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord session: %w", err)
	}

	c.setRunning(true)

	botUser, err := c.session.User("@me")
	if err != nil {
		return fmt.Errorf("failed to get bot user: %w", err)
	}
	c.botUserID = botUser.ID
	logger.InfoCF("discord", "Discord bot connected", map[string]any{
		"username": botUser.Username,
		"user_id":  botUser.ID,
	})

	return nil
}

func (c *DiscordChannel) Stop(ctx context.Context) error {
	logger.InfoC("discord", "Stopping Discord bot")
	c.setRunning(false)

	if err := c.session.Close(); err != nil {
		return fmt.Errorf("failed to close discord session: %w", err)
	}

	return nil
}

func (c *DiscordChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("discord bot not running")
	}

	channelID := msg.ChatID
	if channelID == "" {
		return fmt.Errorf("channel ID is empty")
	}

	runes := []rune(msg.Content)
	if len(runes) == 0 {
		return nil
	}

	chunks := utils.SplitMessage(msg.Content, 2000) // Split messages into chunks, Discord length limit: 2000 chars

	// Determine reply mode from config
	replyToMode := c.config.ReplyToMode
	if replyToMode == "" {
		replyToMode = "first" // Default to "first" for threaded replies
	}

	// Only reply on first chunk if replyToMode is "first", or all chunks if "all"
	replyTo := msg.ReplyTo
	for i, chunk := range chunks {
		var currentReplyTo string
		if replyToMode == "first" {
			// Only reply on first chunk
			if i == 0 {
				currentReplyTo = replyTo
			}
		} else if replyToMode == "all" {
			// Reply on all chunks
			currentReplyTo = replyTo
		}
		// "off" mode never replies

		if err := c.sendChunk(ctx, channelID, chunk, currentReplyTo); err != nil {
			return err
		}
	}

	return nil
}

	defer cancel()

	done := make(chan error, 1)
	go func() {
		var err error
		if replyTo != "" {
			// Send as a reply using message reference
			failIfNotExists := false
			msgRef := discordgo.MessageReference{
				MessageID:       replyTo,
				FailIfNotExists: &failIfNotExists, // Don't fail if the original message was deleted
			}
			_, err = c.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
				Content:   content,
				Reference: &msgRef,
			})
		} else {
			// Send as a regular message
			_, err = c.session.ChannelMessageSend(channelID, content)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.ErrorCF("discord", "Failed to send message chunk", map[string]any{
				"channel_id": channelID,
				"error":      err.Error(),
				"length":     len(content),
				"reply_to":   replyTo,
			})
			return fmt.Errorf("failed to send discord message: %w", err)
		}
		logger.DebugCF("discord", "Message chunk sent successfully", map[string]any{
			"channel_id": channelID,
			"length":     len(content),
			"reply_to":   replyTo,
		})
		return nil
	case <-sendCtx.Done():
		err := fmt.Errorf("send message timeout after 30s")
		logger.ErrorCF("discord", "Send timeout", map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
		})
		return err
	}
}

// appendContent 安全地追加内容到现有文本
func appendContent(content, suffix string) string {
	if content == "" {
		return suffix
	}
	return content + "\n" + suffix
}

func (c *DiscordChannel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Author == nil {
		return
	}

	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check DM policy
	isDirectMessage := m.GuildID == ""
	if isDirectMessage {
		dmPolicy := c.config.DMPolicy
		if dmPolicy == "" {
			dmPolicy = "allowlist" // Default to allowlist
		}

		if dmPolicy == "disabled" {
			logger.DebugCF("discord", "DM dropped (dmPolicy: disabled)", map[string]any{
				"user_id": m.Author.ID,
			})
			return
		}

		if dmPolicy == "allowlist" && !c.IsAllowed(m.Author.ID) {
			logger.DebugCF("discord", "DM dropped (dmPolicy: allowlist, user not allowed)", map[string]any{
				"user_id": m.Author.ID,
			})
			return
		}
		// dmPolicy == "open" allows all DMs
	}

	// 检查白名单，避免为被拒绝的用户下载附件和转录
	// MOVED: Check allowlist BEFORE sending typing indicator
	if !c.IsAllowed(m.Author.ID) {
		logger.DebugCF("discord", "Message rejected by allowlist", map[string]any{
			"user_id": m.Author.ID,
		})
		return
	}

	// Check channel-level allow/deny
	if !isDirectMessage {
		channelConfig, hasConfig := c.resolveChannelConfig(m.GuildID, m.ChannelID)
		if hasConfig && !channelConfig.Allow {
			logger.DebugCF("discord", "Message dropped (channel disabled)", map[string]any{
				"guild_id":   m.GuildID,
				"channel_id": m.ChannelID,
				"user_id":    m.Author.ID,
			})
			return
		}

		// Check role-based access control
		memberRoles := make([]string, 0)
		if m.Member != nil {
			memberRoles = m.Member.Roles
		}
		if !c.isUserAllowedByRoles(m.GuildID, m.Author.ID, memberRoles) {
			logger.DebugCF("discord", "Message dropped (role-based access denied)", map[string]any{
				"guild_id":   m.GuildID,
				"channel_id": m.ChannelID,
				"user_id":    m.Author.ID,
				"roles":      len(memberRoles),
			})
			return
		}

		// Check mention gating
	// Use a longer timeout to prevent race conditions where message is sent but we think it timed out
	// 30 seconds should be enough for Discord API to respond
		requireMention := c.shouldRequireMention(m.GuildID, m.ChannelID)

		if requireMention {
			botMentioned := c.isBotMentioned(m)
			implicitMention := c.isImplicitMention(m)

			if !botMentioned && !implicitMention {
				logger.DebugCF("discord", "Message dropped (mention required)", map[string]any{
					"guild_id":   m.GuildID,
					"channel_id": m.ChannelID,
					"user_id":    m.Author.ID,
				})
				return
			}
			logger.DebugCF("discord", "Message passed mention check", map[string]any{
				"guild_id":         m.GuildID,
				"channel_id":       m.ChannelID,
				"user_id":          m.Author.ID,
				"bot_mentioned":    botMentioned,
				"implicit_mention": implicitMention,
			})
		}
	}

	if err := c.session.ChannelTyping(m.ChannelID); err != nil {
		logger.ErrorCF("discord", "Failed to send typing indicator", map[string]any{
			"error": err.Error(),
		})
		// Don't return - typing indicator failure shouldn't block message processing
	}

	senderID := m.Author.ID
	senderName := m.Author.Username
	if m.Author.Discriminator != "" && m.Author.Discriminator != "0" {
		senderName += "#" + m.Author.Discriminator
	}

	content := m.Content
	mediaPaths := make([]string, 0, len(m.Attachments))
	localFiles := make([]string, 0, len(m.Attachments))

	// 确保临时文件在函数返回时被清理
	defer func() {
		for _, file := range localFiles {
			if err := os.Remove(file); err != nil {
				logger.DebugCF("discord", "Failed to cleanup temp file", map[string]any{
					"file":  file,
					"error": err.Error(),
				})
			}
		}
	}()

	for _, attachment := range m.Attachments {
		isAudio := utils.IsAudioFile(attachment.Filename, attachment.ContentType)

		if isAudio {
			localPath := c.downloadAttachment(attachment.URL, attachment.Filename)
			if localPath != "" {
				localFiles = append(localFiles, localPath)

				transcribedText := ""
				if c.transcriber != nil && c.transcriber.IsAvailable() {
					ctx, cancel := context.WithTimeout(c.getContext(), transcriptionTimeout)
					result, err := c.transcriber.Transcribe(ctx, localPath)
					cancel() // 立即释放context资源，避免在for循环中泄漏

					if err != nil {
						logger.ErrorCF("discord", "Voice transcription failed", map[string]any{
							"error": err.Error(),
						})
						transcribedText = fmt.Sprintf("[audio: %s (transcription failed)]", attachment.Filename)
					} else {
						transcribedText = fmt.Sprintf("[audio transcription: %s]", result.Text)
						logger.DebugCF("discord", "Audio transcribed successfully", map[string]any{
							"text": result.Text,
						})
					}
				} else {
					transcribedText = fmt.Sprintf("[audio: %s]", attachment.Filename)
				}

				content = appendContent(content, transcribedText)
			} else {
				logger.WarnCF("discord", "Failed to download audio attachment", map[string]any{
					"url":      attachment.URL,
					"filename": attachment.Filename,
				})
				mediaPaths = append(mediaPaths, attachment.URL)
				content = appendContent(content, fmt.Sprintf("[attachment: %s]", attachment.URL))
			}
		} else {
			mediaPaths = append(mediaPaths, attachment.URL)
			content = appendContent(content, fmt.Sprintf("[attachment: %s]", attachment.URL))
		}
	}

	if content == "" && len(mediaPaths) == 0 {
		return
	}

	if content == "" {
		content = "[media only]"
	}

	logger.DebugCF("discord", "Received message", map[string]any{
		"sender_name": senderName,
		"sender_id":   senderID,
		"preview":     utils.Truncate(content, 50),
	})

	metadata := map[string]string{
		"message_id":   m.ID,
		"user_id":      senderID,
		"username":     m.Author.Username,
		"display_name": senderName,
		"guild_id":     m.GuildID,
		"channel_id":   m.ChannelID,
		"is_dm":        fmt.Sprintf("%t", m.GuildID == ""),
	}

	c.HandleMessage(senderID, m.ChannelID, content, mediaPaths, metadata)
}

func (c *DiscordChannel) downloadAttachment(url, filename string) string {
	return utils.DownloadFile(url, filename, utils.DownloadOptions{
		LoggerPrefix: "discord",
	})
}

// isBotMentioned checks if the bot was explicitly mentioned in the message
func (c *DiscordChannel) isBotMentioned(m *discordgo.MessageCreate) bool {
	if c.botUserID == "" {
		return false
	}
	// Check mentioned users
	for _, user := range m.Mentions {
		if user.ID == c.botUserID {
			return true
		}
	}
	// Check @everyone mentions
	if m.MentionEveryone {
		return true
	}
	// Check mentioned roles
	if m.GuildID != "" && len(m.MentionRoles) > 0 {
		// We'd need to check if the bot has any of these roles, but for simplicity
		// we'll treat any role mention as a potential mention
		return true
	}
	return false
}

// isImplicitMention checks if this is a reply to the bot's message
func (c *DiscordChannel) isImplicitMention(m *discordgo.MessageCreate) bool {
	if c.botUserID == "" || m.ReferencedMessage == nil {
		return false
	}
	return m.ReferencedMessage.Author.ID == c.botUserID
}

// shouldRequireMention determines if mention is required based on config and guild settings
func (c *DiscordChannel) shouldRequireMention(guildID, channelID string) bool {
	// Default to global setting
	requireMention := c.config.RequireMention

	// Check guild-specific settings
	if guildID != "" {
		if guildConfig, exists := c.config.Guilds[guildID]; exists {
			// Guild-level setting takes precedence
			requireMention = guildConfig.RequireMention

			// Check channel-specific settings
			if channelID != "" {
				// Look up by channel ID
				if channelConfig, exists := guildConfig.Channels[channelID]; exists {
					// Channel-level setting takes precedence
					requireMention = channelConfig.RequireMention
				}
			}
		}
	}

	return requireMention
}

// resolveChannelConfig resolves the channel configuration from guild/channel hierarchy
func (c *DiscordChannel) resolveChannelConfig(guildID, channelID string) (config.DiscordChannelConfig, bool) {
	// Default: allow all channels
	defaultConfig := config.DiscordChannelConfig{Allow: true}

	if guildID == "" {
		// DM - use default
		return defaultConfig, false
	}

	guildConfig, guildExists := c.config.Guilds[guildID]
	if !guildExists {
		// No guild config - use default
		return defaultConfig, false
	}

	if channelID == "" {
		// Guild-level but no channel specified - use default
		return defaultConfig, false
	}

	// Check channel-specific config
	channelConfig, channelExists := guildConfig.Channels[channelID]
	if !channelExists {
		// No channel config - use default
		return defaultConfig, false
	}

	return channelConfig, true
}

// isUserAllowedByRoles checks if the user is allowed based on role configuration
func (c *DiscordChannel) isUserAllowedByRoles(guildID, userID string, memberRoles []string) bool {
	if guildID == "" {
		// DM - no roles to check
		return true
	}

	guildConfig, guildExists := c.config.Guilds[guildID]
	if !guildExists {
		// No guild config - allow all
		return true
	}

	// Check each channel config to see if user has allowed roles
	for _, channelConfig := range guildConfig.Channels {
		if len(channelConfig.Roles) == 0 {
			continue // No role restriction on this channel
		}

		// Check if user has any of the allowed roles
		for _, userRole := range memberRoles {
			for _, allowedRole := range channelConfig.Roles {
				if userRole == allowedRole {
					return true // User has an allowed role
				}
			}
		}
	}

	// If we have guild config with roles but no match, check if there are user restrictions
	for _, channelConfig := range guildConfig.Channels {
		if len(channelConfig.Users) > 0 {
			// User whitelist exists - check if user is in it
			for _, allowedUser := range channelConfig.Users {
				if allowedUser == userID {
					return true
				}
			}
		}
	}

	// If there are role restrictions but user doesn't have any allowed roles, deny
	// But only if there's at least one channel with role restrictions
	hasRoleRestrictions := false
	for _, channelConfig := range guildConfig.Channels {
		if len(channelConfig.Roles) > 0 {
			hasRoleRestrictions = true
			break
		}
	}

	// If there are no role restrictions at all, allow
	if !hasRoleRestrictions {
		return true
	}

	return false
}
