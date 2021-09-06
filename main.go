package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/chrislgardner/AdventureGuild.Discord/oteldiscordgo"
	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

func main() {

	ctx, tp := initHoneycomb()
	// Handle this error in a sensible manner where possible
	defer func() { _ = tp.Shutdown(ctx) }()

	// Open a simple Discord session
	token := os.Getenv("DISCORD_TOKEN")
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}
	err = session.Open()
	if err != nil {
		panic(err)
	}

	// Wait for the user to cancel the process
	defer func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-sc
	}()

	session.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	session.AddHandler(routMessage)
}

func initHoneycomb() (context.Context, *sdktrace.TracerProvider) {
	ctx := context.Background()

	// Create an OTLP exporter, passing in Honeycomb credentials as environment variables.
	exp, err := otlp.NewExporter(
		ctx,
		otlpgrpc.NewDriver(
			otlpgrpc.WithEndpoint("api.honeycomb.io:443"),
			otlpgrpc.WithHeaders(map[string]string{
				"x-honeycomb-team":    os.Getenv("HONEYCOMB_KEY"),
				"x-honeycomb-dataset": os.Getenv("HONEYCOMB_DATASET"),
			}),
			otlpgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
		),
	)

	if err != nil {
		fmt.Printf("failed to initialize exporter: %v", err)
	}

	// Create a new tracer provider with a batch span processor and the otlp exporter.
	// Add a resource attribute service.name that identifies the service in the Honeycomb UI.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(semconv.ServiceNameKey.String("AdventureGuild.Discord"))),
	)

	// Set the Tracer Provider and the W3C Trace Context propagator as globals
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	return ctx, tp
}

func routMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	ctx, span := oteldiscordgo.StartSpanOrTraceFromMessage(s, m.Message)
	defer span.End()

	m.Content = strings.Replace(m.Content, "!", "", 1)
	split := strings.SplitAfterN(m.Content, " ", 2)
	command := strings.Trim(strings.ToLower(split[0]), " ")
	if len(split) == 2 {
		m.Content = split[1]
	}

	span.SetAttributes(attribute.String("parsedCommand", command), attribute.String("remainingContent", m.Content))

	if command == "help" {

	} else if command == "job" {
		resp, err := addJob(ctx, m.Message)
		if err != nil {
			span.SetAttributes(attribute.Any("Error", err))
			sendResponse(ctx, s, m.ChannelID, err.Error())
		}
		sendResponse(ctx, s, m.ChannelID, resp)
	}
}

func sendResponse(ctx context.Context, s *discordgo.Session, cid string, m string) {
	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "SendResponse")
	defer span.End()

	span.SetAttributes(attribute.String("response", m), attribute.String("chennel", cid))

	s.ChannelMessageSend(cid, m)
}

func addJob(ctx context.Context, m *discordgo.Message) (string, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "AddJob")
	defer span.End()

	span.SetAttributes(attribute.String("MessageContent", m.Content))
	return "Done", nil
}
