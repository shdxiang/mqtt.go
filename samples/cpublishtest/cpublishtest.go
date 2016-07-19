package main

import (
	"log"
	"flag"
	"time"
	"strconv"
	"sync"
	MQTT "github.com/shdxiang/mqtt.go"
)

var msgSent int = 0
var msgRecv int = 0
var wgSub sync.WaitGroup

func defaultPublishHandler(client *MQTT.MqttClient, msg MQTT.Message) {
	log.Printf("======TOPIC: %s\n", msg.Topic())
	log.Printf("======MSG: %s\n", msg.Payload())
}

func onMessageReceived(client *MQTT.MqttClient, message MQTT.Message) {
	log.Printf("======Received message on topic: %s\n", message.Topic())
	log.Printf("======Message: %s\n", message.Payload())
	msgRecv++
}

func doWork(index int, appkey *string, topic *string, qos int, message *string, pubCnt int, interval int) {
	deviceId := strconv.Itoa(time.Now().Second()) + strconv.Itoa(index)

	yunbaClient := &MQTT.YunbaClient{*appkey, deviceId}
	regInfo, err := yunbaClient.Reg()
	if err != nil {
		log.Fatal(err)
	}

	if regInfo.ErrCode != 0 {
		log.Fatal("has error:", regInfo.ErrCode)
	}

	broker := "tcp://123.56.125.40:1883"

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(broker)
	connOpts.SetClientId(regInfo.Client)
	connOpts.SetCleanSession(true)
	connOpts.SetProtocolVersion(0x13)

	connOpts.SetUsername(regInfo.UserName)
	connOpts.SetPassword(regInfo.Password)

	connOpts.SetDefaultPublishHandler(defaultPublishHandler)

	client := MQTT.NewClient(connOpts)
	_, err = client.Start()
	if err != nil {
		panic(err)
	} else {
		log.Printf("Connected to %s\n", broker)
	}

	// <- client.SetAlias(deviceId)

	filter, e := MQTT.NewTopicFilter(*topic, byte(qos))
	if e != nil {
		log.Fatal(e)
	}

	client.StartSubscription(onMessageReceived, filter)
	// client.Presence(onMessageReceived, *topic)
	wgSub.Done()
	wgSub.Wait()

	time.Sleep(5 * time.Second)

	for i := 0; i < pubCnt; i++ {
		<- client.Publish(MQTT.QoS(qos), *topic, []byte(*message))
		log.Printf("======Published\n")
		time.Sleep(time.Duration(interval) * time.Millisecond)
	}
}

func main() {
	appkey := flag.String("appkey", "", "YunBa appkey")
	topic := flag.String("topic", "", "Topic to publish the messages on")
	message := flag.String("message", "hello", "Message to be published")
	qos := flag.Int("qos", 0, "The QoS to send the messages at")

	routineNum  := flag.Int("routine", 1, "Number of goroutine")
	ecahPubCnt := flag.Int("each", 1, "Each routine publish count")
	interval := flag.Int("interval", 1000, "Publish interval for a routine, millisecond")
	//retained := flag.Bool("retained", false, "Are the messages sent with the retained flag")
	flag.Parse()

	if *appkey == "" {
		log.Fatal("please set appkey")
	}

	if *topic == "" {
		log.Fatal("please set topic")
	}

	for i := 0; i < *routineNum; i++ {
		wgSub.Add(1)
		go doWork(i, appkey, topic, *qos, message, *ecahPubCnt, *interval)
		log.Printf("======Add: %d\n", i)
		time.Sleep(10 * time.Millisecond)
	}

	msgNeedRecv := (*routineNum * *ecahPubCnt * *routineNum)

	for {
		time.Sleep(2 * time.Second)
		if msgRecv == msgNeedRecv {
			break
		}
		log.Printf("======msgRecv: %d\n", msgRecv)
	}
	log.Printf("======msgRecv: %d\n", msgRecv)
}
