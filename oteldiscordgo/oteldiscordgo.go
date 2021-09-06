package oteldiscordgo

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StartSpanOrTraceFromMessage creates and returns a span for the provided MessageEvent. If
// there is an existing span in the Context, this function will create the new span as a
// child span and return it. If not, it will create a new trace object and return the root
// span.
func StartSpanOrTraceFromMessage(session *discordgo.Session, message *discordgo.Message) (context.Context, trace.Span) {
	// Create a tracer instance.
	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(context.Background(), "MessageRespond")

	span.SetAttributes(getMessageProps(message)...)
	span.SetAttributes(getSessionProps(session)...)

	return ctx, span
}

func getMessageProps(me *discordgo.Message) []attribute.KeyValue {

	messageProps := []attribute.KeyValue{
		attribute.Any("message.ID", me.ID),
		attribute.Any("message.ChannelID", me.ChannelID),
		attribute.Any("message.GuildID", me.GuildID),
		attribute.Any("message.AuthorID", me.Author.ID),
		attribute.Any("message.AuthorUsername", me.Author.Username),
		attribute.Any("message.MessageType", me.Type),
		attribute.Any("message.RawContent", me.Content),
		attribute.Any("message.MentionEveryone", me.MentionEveryone),
		attribute.Any("message.MentionRoles", me.MentionRoles),
	}
	channels := []string{""}
	for _, mc := range me.MentionChannels {
		channels = append(channels, mc.ID)
	}

	messageProps = append(messageProps, attribute.Any("message.MentionChannels", channels))

	mentions := []string{""}
	for _, m := range me.Mentions {
		mentions = append(mentions, m.ID)
	}

	messageProps = append(messageProps, attribute.Any("message.Mentions", mentions))

	return messageProps
}

func getSessionProps(s *discordgo.Session) []attribute.KeyValue {
	sessionProps := []attribute.KeyValue{
		attribute.Any("session.ShardID", s.ShardID),
	}

	return sessionProps
}
