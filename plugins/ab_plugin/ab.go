package ab_plugin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/benthosdev/benthos/v4/public/service"
	"github.com/danomagnum/gologix"
)

//------------------------------------------------------------------------------

// S7CommInput struct defines the structure for our custom Benthos input plugin.
// It holds the configuration necessary to establish a connection with a Siemens S7 PLC,
// along with the read requests to fetch data from the PLC.
type ABCommInput struct {
	tcpDevice        string        // IP address of the S7 PLC.
	timeout          time.Duration // Time duration before a connection attempt or read request times out.
	subscription     []subscriptionDef
	tSubscription    []tSubscriptionsDef
	insecure         bool
	subscribeEnabled bool
	log              *service.Logger // Logger for logging plugin activity.
	client           *gologix.Client
}
type subscriptionDef struct {
	Name              string
	TagName           string
	MqttTopic         string
	LoggingSQL        bool
	SQLStoreProcedure string
	LoggingInflux     bool
	InfluxeBucket     string
	Group             string
	Type              string
	DataType          string
	Condition         string
}
type tSubscriptionsDef struct {
	Trigger string
	Nodes   []Node
}

type Node struct {
	Name     string
	DataType string
	TagName  string
}

func ParseSubscription(subscription []string) []subscriptionDef {
	parsedSubscription := make([]subscriptionDef, len(subscription))

	for _, subscriptionElement := range subscription {

		var subscr subscriptionDef
		err := json.Unmarshal([]byte(subscriptionElement), &subscr)
		if err != nil {
			return nil
		}
		log.Println("Girish ParseSubscription() json tagname: ", subscr.TagName)
		parsedSubscription = append(parsedSubscription, subscr)
	}

	return parsedSubscription
}
func ParseTSubscription(tSubscription []string) []tSubscriptionsDef {
	parsedtSubscription := make([]tSubscriptionsDef, len(tSubscription))

	for _, subscriptionElement := range tSubscription {

		var subscr tSubscriptionsDef
		err := json.Unmarshal([]byte(subscriptionElement), &subscr)
		if err != nil {
			return nil
		}
		log.Println("Girish ParseTSubscription() json tagname: ", subscr.Nodes)
		parsedtSubscription = append(parsedtSubscription, subscr)
	}

	return parsedtSubscription
}

// S7CommConfigSpec defines the configuration options available for the S7CommInput plugin.
// It outlines the required information to establish a connection with the PLC and the data to be read.
var ABCommInputCommConfigSpec = service.NewConfigSpec().
	Summary("Creates an input that reads data from Allen Bradley PLCs. Created & maintained by the").
	Description("This input plugin enables Benthos to read data directly from Allen Bradly PLCs" +
		"Configure the plugin by specifying the PLC's IP address, rack and slot numbers, and the data blocks to read.").
	Field(service.NewStringField("tcpDevice").Description("IP address of the Allen Bradly PLC.")).
	Field(service.NewIntField("timeout").Description("The timeout duration in seconds for connection attempts and read requests.").Default(10)).
	Field(service.NewStringListField("subscriptions").Description("List of AB addresses Address formats include direct area access")).
	Field(service.NewStringListField("tsubscriptions").Description("List of AB trigger node IDs.")).
	Field(service.NewBoolField("insecure").Description("Set to true to bypass secure connections, useful in case of SSL or certificate issues. Default is secure (false).").Default(false)).
	Field(service.NewBoolField("subscribeEnabled").Description("Set to true to subscribe to AB nodes instead of fetching them every seconds. Default is pulling messages every second (false).").Default(false))

// newS7CommInput is the constructor function for S7CommInput. It parses the plugin configuration,
// establishes a connection with the S7 PLC, and initializes the input plugin instance.
func newABCommInput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
	log.Println("Girish newABCommInput()")
	tcpDevice, err := conf.FieldString("tcpDevice")
	if err != nil {
		return nil, err
	}

	timeoutInt, err := conf.FieldInt("timeout")
	if err != nil {
		return nil, err
	}

	subscriptions, err := conf.FieldStringList("subscriptions")
	if err != nil {
		return nil, err
	}

	tsubscriptions, err := conf.FieldStringList("tsubscriptions")
	if err != nil {
		return nil, err
	}
	log.Println(tsubscriptions)

	insecure, err := conf.FieldBool("insecure")
	if err != nil {
		return nil, err
	}

	subscribeEnabled, err := conf.FieldBool("subscribeEnabled")
	if err != nil {
		return nil, err
	}
	sub := ParseSubscription(subscriptions)
	tSub := ParseTSubscription(tsubscriptions)

	m := &ABCommInput{
		tcpDevice:        tcpDevice,
		subscription:     sub,
		tSubscription:    tSub,
		insecure:         insecure,
		subscribeEnabled: subscribeEnabled,
		log:              mgr.Logger(),
		timeout:          time.Duration(timeoutInt) * time.Second,
	}

	return service.AutoRetryNacksBatched(m), nil
}

//------------------------------------------------------------------------------

func init() {
	log.Println("Girish init()")
	err := service.RegisterBatchInput(
		"abcomm", ABCommInputCommConfigSpec,
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
			return newABCommInput(conf, mgr)
		})
	if err != nil {
		panic(err)
	}
}

func (g *ABCommInput) Connect(ctx context.Context) error {
	log.Println("Girish Connect()")
	if g.client != nil {
		return nil
	}
	client := gologix.NewClient(g.tcpDevice)
	err := client.Connect()
	if err != nil {
		log.Printf("Error opening client. %v", err)
		return err
	}
	g.client = client
	return nil
}

func (g *ABCommInput) ReadBatch(ctx context.Context) (service.MessageBatch, service.AckFunc, error) {
	log.Println("Girish ReadBatch()")
	if ctx == nil || ctx.Done() == nil {
		return nil, nil, errors.New("emptyCtx is invalid for ReadBatchSubscribe")
	}
	err := g.client.ListAllTags(0)

	if err != nil {
		log.Printf("Error getting tag list. %v", err)
		return nil, func(ctx context.Context, err error) error {
			return nil // Acknowledgment handling here if needed
		}, err
	}

	log.Printf("Found %d tags.", len(g.client.KnownTags))
	// list through the tag list and read them all
	for tagname := range g.client.KnownTags {
		tag := g.client.KnownTags[tagname]
		log.Printf("%s: %v", tag.Name, tag.Info.Type)

		// TODO: in theory we should do more to read multi-dim arrays.
		qty := uint16(1)
		if tag.Info.Dimension1 != 0 {
			tagname = tagname + "[0]"
			x := tag.Info.Atomic()
			qty = uint16(tag.Info.Dimension1)
			_ = x
		}
		if tag.UDT == nil && !tag.Info.Atomic() {
			//log.Print("Not Atomic or UDT")
			continue
		}
		if tag.UDT != nil {
			log.Printf("%s size = %d", tag.Name, tag.UDT.Size())
		}

		val, err := g.client.Read_single(tagname, tag.Info.Type, qty)
		if err != nil {
			log.Printf("Error!  Problem reading tag %s. %v", tagname, err)
			continue
		}
		log.Printf("     = %v", val)
	}

	log.Printf("Found %d tags.", len(g.client.KnownTags))

	return nil, func(ctx context.Context, err error) error {
		return nil // Acknowledgment handling here if needed
	}, nil
}

func (g *ABCommInput) Close(ctx context.Context) error {
	log.Println("Girish Close()")
	if g.client != nil {
		g.client.Disconnect()
	}
	return nil
}

// createMessageFromValue creates a benthos messages from a given variant and nodeID
// theoretically nodeID can be extracted from variant, but not in all cases (e.g., when subscribing), so it it left to the calling function
/* func (g *ABCommInput) createMessageFromValue(tag *gologix.KnownTag) *service.Message {

	if tag == nil {
		g.log.Errorf("tag is nil")
		return nil
	}

	b := make([]byte, 0)

	message.MetaSet("Message", string(jsonMsg))
	return message
} */
