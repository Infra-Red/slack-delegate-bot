package slack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dpb587/slack-delegate-bot/pkg/message"
	"github.com/slack-go/slack"
)

type MessageParser struct {
	self             *slack.UserDetails
	reMention        *regexp.Regexp
	reChannelMention *regexp.Regexp
}

func NewMessageParser(self *slack.UserDetails) *MessageParser {
	return &MessageParser{
		self:             self,
		reMention:        regexp.MustCompile(fmt.Sprintf(`<@%s>`, regexp.QuoteMeta(self.ID))),
		reChannelMention: regexp.MustCompile(fmt.Sprintf(`<#([^|]+)\|([^>]+)>\s+<@%s>`, regexp.QuoteMeta(self.ID))),
	}
}

func (p *MessageParser) ParseMessage(msg slack.Msg) (*message.Message, error) {
	if msg.Type != "message" {
		return nil, nil
	} else if msg.SubType == "message_deleted" {
		// no sense responding to deleted message notifications
		return nil, nil
	} else if msg.SubType == "group_topic" || strings.Contains(msg.Text, "set the channel topic: ") {
		// no sense responding to a reference in the topic
		// trivia: slack doesn't support topic threads, but still allows bots to
		// respond which means you get mentioned, but the browser app doesn't
		// render the thread in New Threads so you can't mark it as read unless you
		// use the mobile app (which happens to show it as -1 replies).
		return nil, nil
	} else if msg.User == p.self.ID {
		// avoid accidentally talking to ourselves into a recursive DoS
		return nil, nil
	}

	incoming := &message.Message{
		Origin:          msg.Channel,
		OriginType:      message.ChannelOriginType,
		InterruptTarget: msg.Channel,
		Timestamp:       convertSlackTimestamp(msg.Timestamp),
		Text:            msg.Text,
	}

	// include attachments
	for _, attachment := range msg.Attachments {
		if attachment.Fallback == "" {
			continue
		}

		incoming.Text = fmt.Sprintf("%s\n\n---\n\n%s", incoming.Text, attachment.Fallback)
	}

	if msg.Channel[0] == 'D' { // TODO better way to detect if this is our bot DM?
		matches := reChannelMention.FindStringSubmatch(incoming.Text)
		if len(matches) > 0 {
			incoming.InterruptTarget = matches[1]
		}

		incoming.OriginType = message.DirectMessageOriginType

		return incoming, nil
	} else if !p.reMention.MatchString(incoming.Text) {
		return nil, nil
	}

	matches := p.reChannelMention.FindStringSubmatch(incoming.Text)
	if len(matches) > 0 {
		incoming.InterruptTarget = matches[1]
	}

	return incoming, nil
}
