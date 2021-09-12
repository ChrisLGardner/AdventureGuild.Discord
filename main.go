package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/chrislgardner/AdventureGuild.Discord/oteldiscordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv"
)

type Reminder struct {
	Server          string
	Channel         string
	Creator         string
	Message         string
	JobBoardMessage string
	Job             Job
	Date            time.Time
	CreatedDate     time.Time
}

type Job struct {
	Title         string
	Date          time.Time
	Creator       *discordgo.User
	Description   string
	Server        string
	Channel       string
	CreatedDate   time.Time
	SourceChannel string
}

var (
	destinationChannel string
	messageTemplate    string = `
Hi %s,

Just a friendly reminder that you've got a session on %s for the adventure titled %s. More details are here: %s
	`
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

	go sendReminders(session)
	session.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	destinationChannel = "884786622846099466"
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
	if strings.Contains(split[0], "\n") {
		split = strings.SplitAfterN(m.Content, "\n", 2)
	}
	command := strings.Trim(strings.Trim(strings.ToLower(split[0]), " "), "\n")
	if len(split) == 2 {
		m.Content = split[1]
	}

	span.SetAttributes(attribute.String("parsedCommand", command), attribute.String("remainingContent", m.Content))

	if command == "help" {

	} else if command == "job" {
		resp, err := addJob(ctx, m.Message, s)
		if err != nil {
			span.SetAttributes(attribute.Any("Error", err))
			sendResponse(ctx, s, m.ChannelID, err.Error())
			return
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

func sendJob(ctx context.Context, s *discordgo.Session, cid string, job Job) (*discordgo.Message, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "SendJob")
	defer span.End()

	span.SetAttributes(attribute.Any("response", job), attribute.String("channel", cid))

	jobEmbed := formatJobEmbed(ctx, job)

	res, err := s.ChannelMessageSendEmbed(cid, jobEmbed)
	if err != nil {
		span.SetAttributes(attribute.String("SendJob.Error", err.Error()))
		return &discordgo.Message{}, err
	}

	return res, nil

}

func addJob(ctx context.Context, m *discordgo.Message, s *discordgo.Session) (string, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "AddJob")
	defer span.End()

	span.SetAttributes(attribute.String("MessageContent", m.Content))

	var job Job
	var err error
	if len(m.Attachments) == 1 {
		span.SetAttributes(attribute.Int("Attachments", 1))
		job, err = parseJobAttachment(ctx, m)
		if err != nil {
			span.SetAttributes(attribute.String("AddJob.Error", err.Error()))
			return "", err
		}
	} else if len(m.Attachments) == 0 {
		job, err = parseJobMessage(ctx, m)
		if err != nil {
			span.SetAttributes(attribute.String("AddJob.Error", err.Error()))
			return "", err
		}
	} else {
		span.SetAttributes(attribute.Int("Attachments", len(m.Attachments)))
		span.SetAttributes(attribute.String("AddJob.Error", "Too many attachments on message"))
		return "", fmt.Errorf("too many attachments (%d) on message", len(m.Attachments))
	}

	JobMessage, err := sendJob(ctx, s, destinationChannel, job)
	if err != nil {
		span.SetAttributes(attribute.String("AddJob.Error", err.Error()))
		return "", err
	}

	err = createReminder(ctx, job, JobMessage)
	if err != nil {
		span.SetAttributes(attribute.String("AddJob.Error", err.Error()))
		return "", err
	}

	return job.Title, nil
}

func parseJobAttachment(ctx context.Context, m *discordgo.Message) (Job, error) {
	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "ParseJobAttachment")
	defer span.End()

	span.SetAttributes(attribute.String("createSpell.Attachment.Url", m.Attachments[0].URL), attribute.String("createSpell.Attachment.Filename", m.Attachments[0].Filename))

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   time.Second * 20,
	}
	getReq, _ := http.NewRequestWithContext(ctx, "GET", m.Attachments[0].URL, nil)
	resp, err := client.Do(getReq)
	if err != nil {
		span.SetAttributes(attribute.String("ParseJobAttachment.Error", err.Error()))
		return Job{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		span.SetAttributes(attribute.String("ParseJobAttachment.Error", err.Error()))
		return Job{}, err
	}
	_ = resp.Body.Close()

	rawResponse := string(body)
	span.SetAttributes(attribute.String("ParseJobAttachment.RawContent", rawResponse))
	trimmedContent := strings.Replace(strings.Replace(rawResponse, "\r", "", -1), "\x0d", "", -1)
	createdDate, _ := m.Timestamp.Parse()

	job := Job{
		Creator:       m.Author,
		Server:        m.GuildID,
		Channel:       destinationChannel,
		CreatedDate:   createdDate,
		SourceChannel: m.ChannelID,
	}

	err = job.parseString(trimmedContent)
	if err != nil {
		span.SetAttributes(attribute.String("ParseJobAttachment.Error", err.Error()))
		return Job{}, err
	}

	span.SetAttributes(
		attribute.String("ParseJobAttachment.Job.Title", job.Title),
		attribute.String("ParseJobAttachment.Job.Creator", job.Creator.ID),
		attribute.Any("ParseJobAttachment.Job.Date", job.Date),
		attribute.String("ParseJobAttachment.Job.Description", job.Description),
		attribute.String("ParseJobAttachment.Job.Server", job.Server),
		attribute.String("ParseJobAttachment.Job.Channel", job.Channel),
		attribute.Any("ParseJobAttachment.Job.CreatedDate", job.CreatedDate),
		attribute.String("ParseJobAttachment.Job.SourceChannel", job.SourceChannel),
	)

	return job, nil
}

func parseJobMessage(ctx context.Context, m *discordgo.Message) (Job, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "ParseJobMessage")
	defer span.End()

	content := strings.Replace(m.Content, "job ", "", 1)
	createdDate, _ := m.Timestamp.Parse()

	job := Job{
		Creator:       m.Author,
		Server:        m.GuildID,
		Channel:       destinationChannel,
		CreatedDate:   createdDate,
		SourceChannel: m.ChannelID,
	}

	err := job.parseString(content)
	if err != nil {
		span.SetAttributes(attribute.String("ParseJobMessage.Error", err.Error()))
		return Job{}, err
	}

	span.SetAttributes(
		attribute.String("ParseJobMessage.Job.Title", job.Title),
		attribute.String("ParseJobMessage.Job.Creator", job.Creator.ID),
		attribute.Any("ParseJobMessage.Job.Date", job.Date),
		attribute.String("ParseJobMessage.Job.Description", job.Description),
		attribute.String("ParseJobMessage.Job.Server", job.Server),
		attribute.String("ParseJobMessage.Job.Channel", job.Channel),
		attribute.Any("ParseJobMessage.Job.CreatedDate", job.CreatedDate),
		attribute.String("ParseJobMessage.Job.SourceChannel", job.SourceChannel),
	)

	return job, nil
}

func formatJobEmbed(ctx context.Context, job Job) *discordgo.MessageEmbed {
	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "FormatJobEmbed")
	defer span.End()

	span.SetAttributes(attribute.Any("FormatJobEmbed.Job", job))

	embedFields := []*discordgo.MessageEmbedField{
		{
			Name:   "DM",
			Value:  fmt.Sprintf("<@!%s>", job.Creator.ID),
			Inline: true,
		},
		{
			Name:   "Date",
			Value:  job.Date.Format("2006-01-02 15:04"),
			Inline: true,
		},
	}
	jobEmbed := discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       job.Title,
		Description: job.Description,
		Fields:      embedFields,
	}

	return &jobEmbed
}

func (j *Job) parseString(s string) error {
	content := strings.Split(s, "\n")
	if strings.Trim(content[0], " ") == "" {
		content = content[1:]
	}
	date, err := parseDate(content[1])
	if err != nil {
		return err
	}

	j.Title = strings.Trim(content[0], " ")
	j.Date = date
	j.Description = strings.Join(content[2:], "\n")

	return nil
}

func parseDate(s string) (time.Time, error) {

	acceptedFormats := []string{
		"2006-01-02 15:04",
		"2006-01-02 3:04PM",
		"2006/01/02 15:04",
		"2006/01/02 3:04PM",
		"01-02-2006 15:04",
		"01-02-2006 3:04PM",
		"01/02/2006 15:04",
		"01/02/2006 3:04PM",
	}

	for _, format := range acceptedFormats {
		date, err := time.Parse(format, s)
		if err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format provided, please provide date in the format Year-Month-Day 24:00")
}

func createReminder(ctx context.Context, job Job, jobMessage *discordgo.Message) error {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "CreateReminder")
	defer span.End()

	reminder := Reminder{
		Server:          job.Server,
		Channel:         job.Channel,
		Creator:         job.Creator.ID,
		Message:         messageTemplate,
		Job:             job,
		Date:            job.Date,
		CreatedDate:     time.Now(),
		JobBoardMessage: jobMessage.ID,
	}

	span.SetAttributes(attribute.Any("CreateReminder.Reminder", reminder))

	err := storeReminder(ctx, reminder)
	if err != nil {
		span.SetAttributes(attribute.String("CreateReminder.Error", err.Error()))
		return err
	}
	return nil
}

func storeReminder(ctx context.Context, reminder Reminder) error {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "StoreReminder")
	defer span.End()
	span.SetAttributes(attribute.Any("StoreReminder.Reminder", reminder))

	db, err := connectDb(ctx, os.Getenv("COSMOSDB_URI"))
	if err != nil {
		span.SetAttributes(attribute.String("StoreReminder.Error", err.Error()))
		return err
	}

	err = writeDbObject(ctx, db, reminder)
	if err != nil {
		span.SetAttributes(attribute.String("StoreReminder.Error", err.Error()))
		return err
	}

	if err = db.Disconnect(ctx); err != nil {
		span.SetAttributes(attribute.String("StoreReminder.Error", err.Error()))
		return err
	}

	return nil
}

func connectDb(ctx context.Context, uri string) (*mongo.Client, error) {
	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "ConnectDB")
	defer span.End()

	span.SetAttributes(attribute.String("ConnectDB.mongo.server", uri))

	clientOptions := options.Client().ApplyURI(uri).SetDirect(true)
	c, err := mongo.NewClient(clientOptions)
	if err != nil {
		span.SetAttributes(attribute.String("ConnectDB.mongo.client.error", err.Error()))
		return nil, err
	}

	err = c.Connect(ctx)
	if err != nil {
		span.SetAttributes(attribute.String("ConnectDB.mongo.connect.error", err.Error()))
		return nil, err
	}

	err = c.Ping(ctx, nil)
	if err != nil {
		span.SetAttributes(attribute.String("ConnectDB.mongo.ping.error", err.Error()))
		return nil, err
	}

	return c, nil
}

func writeDbObject(ctx context.Context, mc *mongo.Client, obj interface{}) error {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "WriteDBObject")
	defer span.End()

	data, err := bson.Marshal(obj)
	if err != nil {
		span.SetAttributes(attribute.String("WriteDBObject.mongo.error", err.Error()))
		return err
	}

	collection := mc.Database("reminders").Collection("reminders")
	span.SetAttributes(attribute.String("WriteDBObject.mongo.collection", collection.Name()))
	span.SetAttributes(attribute.String("WriteDBObject.mongo.database", collection.Database().Name()))
	span.SetAttributes(attribute.Any("WriteDBObject.mongo.object", data))

	res, err := collection.InsertOne(ctx, data)
	if err != nil {
		span.SetAttributes(attribute.String("WriteDBObject.mongo.error", err.Error()))
		return err
	}

	span.SetAttributes(attribute.Any("WriteDBObject.mongo.id", res.InsertedID))

	return nil
}

func runQuery(ctx context.Context, mc *mongo.Client, query interface{}) ([]bson.M, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "RunQuery")
	defer span.End()

	collection := mc.Database("reminders").Collection("reminders")
	span.SetAttributes(attribute.String("RunQuery.mongo.collection", collection.Name()))
	span.SetAttributes(attribute.String("RunQuery.mongo.database", collection.Database().Name()))
	span.SetAttributes(attribute.Any("RunQuery.mongo.query", query))

	cursor, err := collection.Find(ctx, query)
	if err != nil {
		span.SetAttributes(attribute.String("RunQuery.mongo.error", err.Error()))
		return nil, err
	}

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		span.SetAttributes(attribute.String("RunQuery.mongo.error", err.Error()))
		return nil, err
	}

	span.SetAttributes(attribute.Int("RunQuery.mongo.results.Count", len(results)))
	span.SetAttributes(attribute.Any("RunQuery.mongo.results.raw", results))

	return results, nil
}

func sendReminders(session *discordgo.Session) {
	storage := os.Getenv("COSMOSDB_URI")
	interval, err := strconv.Atoi(os.Getenv("REMINDER_INTERVAL"))
	if err != nil {
		interval = 24
	}
	for {

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

		tracer := otel.Tracer("AdventureGuild.Discord")
		ctx, span := tracer.Start(ctx, "SendReminders")

		db, err := connectDb(ctx, storage)
		if err != nil {
			span.SetAttributes(attribute.String("sendReminders.connect.error", err.Error()))
			span.End()
			continue
		}

		reminders, err := findReminders(ctx, db, interval)
		if err != nil {
			span.SetAttributes(attribute.String("sendReminders.find.error", err.Error()))
			span.End()
			continue
		}

		for _, r := range reminders {
			ctx, childSpan := tracer.Start(ctx, "SendIndividualReminder")
			childSpan.SetAttributes(
				attribute.String("SendIndividualReminder.Date", r.Date.Format(time.RFC3339)),
				attribute.String("SendIndividualReminder.Message", r.Message),
				attribute.String("SendIndividualReminder.Server", r.Server),
				attribute.String("SendIndividualReminder.Creator", r.Creator),
				attribute.String("SendIndividualReminder.Channel", r.Channel),
				attribute.String("SendIndividualReminder.JobBoardMessage", r.JobBoardMessage),
				attribute.String("SendIndividualReminder.CreatedDate", r.CreatedDate.Format(time.RFC3339)),
				attribute.Any("SendIndividualReminder.Job", r.Job),
			)

			users, err := getReactedUsers(ctx, session, r.Server, r.Channel, r.JobBoardMessage)
			if err != nil {
				childSpan.SetAttributes(attribute.String("SendIndividualReminder.find.error", err.Error()))
				childSpan.End()
				continue
			}

			childSpan.SetAttributes(attribute.Any("SendIndividualReminder.Users", users))
			sourceMessageUrl := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", r.Server, r.Channel, r.JobBoardMessage)

			for _, user := range users {
				message := fmt.Sprintf(r.Message, user.Username, r.Date.Format("2006-01-02 15:04"), r.Job.Title, sourceMessageUrl)

				sendUserDM(ctx, session, message, user.ID)
			}

			childSpan.End()
		}

		err = db.Disconnect(ctx)
		if err != nil {
			span.SetAttributes(attribute.String("SendReminders.Error", err.Error()))
			span.End()
			continue
		}

		span.End()
		time.Sleep(time.Duration(interval) * time.Hour)
	}
}

func findReminders(ctx context.Context, db *mongo.Client, interval int) ([]Reminder, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "FindReminders")
	defer span.End()

	start := time.Now().UnixNano() / 1000000
	end := time.Now().Add(time.Duration(interval)*time.Hour).UnixNano() / 1000000
	query := bson.M{
		"date": bson.M{
			"$gt": start,
			"$lt": end,
		},
	}

	span.SetAttributes(attribute.Any("FindReminders.query", query))

	res, err := runQuery(ctx, db, query)
	if err != nil {
		span.SetAttributes(attribute.String("FindReminders.error", err.Error()))
		return nil, err
	}

	var reminders []Reminder
	for _, item := range res {
		var r Reminder

		temp, err := bson.Marshal(item)
		if err != nil {
			span.SetAttributes(attribute.String("findReminders.error", err.Error()))
			return nil, err
		}

		err = bson.Unmarshal(temp, &r)
		if err != nil {
			span.SetAttributes(attribute.String("findReminders.error", err.Error()))
			return nil, err
		}

		reminders = append(reminders, r)
	}

	return reminders, nil
}

func getReactedUsers(ctx context.Context, session *discordgo.Session, server, channel, message string) ([]*discordgo.User, error) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "GetReactedUsers")
	defer span.End()

	channelMessage, err := session.ChannelMessages(channel, 1, "", "", message)
	if err != nil {
		span.SetAttributes(attribute.String("GetReactedUsers.ChannelMessage.Error", err.Error()))
		return nil, err
	}
	span.SetAttributes(attribute.Any("GetReactedUsers.ChannelMessage", channelMessage[0]), attribute.Int("GetReactedUsers.ChannelMessageCount", len(channelMessage)))

	reactedUsersMap := make(map[string]bool)
	reactedUsers := []*discordgo.User{}

	for _, emoji := range channelMessage[0].Reactions {

		users, err := session.MessageReactions(channel, channelMessage[0].ID, emoji.Emoji.Name, 100, "", "")
		if err != nil {
			span.SetAttributes(attribute.String("GetReactedUsers.MessageReactions.Error", err.Error()))
			return nil, err
		}
		span.SetAttributes(attribute.Any(fmt.Sprintf("GetReactedUsers.MessageReactions.Emoji.%s.Users", emoji.Emoji.ID), users))
		for _, user := range users {
			if _, ok := reactedUsersMap[user.ID]; !ok {
				reactedUsersMap[user.ID] = true
				reactedUsers = append(reactedUsers, user)
			}
		}
	}

	span.SetAttributes(attribute.Any("GetReactedUsers.Users", reactedUsers))
	return reactedUsers, nil
}

func sendUserDM(ctx context.Context, session *discordgo.Session, message, userid string) {

	tracer := otel.Tracer("AdventureGuild.Discord")
	ctx, span := tracer.Start(ctx, "sendUserDM")
	defer span.End()

	span.SetAttributes(attribute.String("SendUserDM.Message", message), attribute.String("SendUserDM.UserID", userid))
	channel, err := session.UserChannelCreate(userid)
	if err != nil {
		span.SetAttributes(attribute.String("sendUserDM.error", err.Error()))
	}
	span.SetAttributes(attribute.String("SendUserDM.Channel", channel.ID))

	session.ChannelMessageSend(channel.ID, message)
}
