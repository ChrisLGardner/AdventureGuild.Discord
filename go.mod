module github.com/chrislgardner/AdventureGuild.Discord

go 1.16

require (
	github.com/bwmarrin/discordgo v0.23.2
	go.mongodb.org/mongo-driver v1.7.3
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.25.0
	go.opentelemetry.io/otel v1.0.1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.1
	go.opentelemetry.io/otel/sdk v1.0.1
	go.opentelemetry.io/otel/trace v1.0.1
	google.golang.org/grpc v1.41.0
)
