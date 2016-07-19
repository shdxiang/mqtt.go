package main

import (
	"os"
	"bufio"
	"log"
	"flag"
	"time"
	"strconv"
	"sync"
	"strings"
	MQTT "github.com/shdxiang/mqtt.go"
)

var msgSent int = 0
var msgRecv int = 0
var wg sync.WaitGroup

func defaultPublishHandler(client *MQTT.MqttClient, msg MQTT.Message) {
	log.Printf("TOPIC: %s\n", msg.Topic())
	log.Printf("MSG: %s\n", msg.Payload())
}

func onMessageReceived(client *MQTT.MqttClient, message MQTT.Message) {
	log.Printf("Received message on topic: %s\n", message.Topic())
	log.Printf("Message: %s\n", message.Payload())
	msgRecv++
}

func doReg(index int, regFile *os.File, appkey *string, topic *string, qos int, broker *string) {
	deviceId := strconv.Itoa(time.Now().Second()) + strconv.Itoa(index)

	yunbaClient := &MQTT.YunbaClient{*appkey, deviceId}
	regInfo, err := yunbaClient.Reg()
	if err != nil {
		log.Fatal(err)
	}

	if regInfo.ErrCode != 0 {
		log.Fatal("has error:", regInfo.ErrCode)
	}

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(*broker)
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
		log.Printf("Connected to %s\n", *broker)
	}

	filter, err := MQTT.NewTopicFilter(*topic, byte(qos))
	if err != nil {
		log.Fatal(err)
	}

	client.StartSubscription(onMessageReceived, filter)

	line := strings.Join([]string{regInfo.Client, regInfo.UserName, regInfo.Password}, "|")

	log.Printf("line: %s\n", line)
	writer := bufio.NewWriter(regFile)
    _, err = writer.WriteString(line + "\n")
    if err != nil {
    	log.Fatal(err)
    }

    writer.Flush()
	wg.Done()
}

func doWork(index int, clientid *string, user *string, pass *string, broker *string, topic *string, qos int, message *string, pubCnt int, interval int) {
	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(*broker)
	connOpts.SetClientId(*clientid)
	connOpts.SetCleanSession(true)
	connOpts.SetProtocolVersion(0x13)

	connOpts.SetUsername(*user)
	connOpts.SetPassword(*pass)

	connOpts.SetDefaultPublishHandler(defaultPublishHandler)

	client := MQTT.NewClient(connOpts)
	_, err := client.Start()
	if err != nil {
		panic(err)
	} else {
		log.Printf("Connected to %s\n", *broker)
	}

	// <- client.SetAlias(deviceId)

	filter, e := MQTT.NewTopicFilter(*topic, byte(qos))
	if e != nil {
		log.Fatal(e)
	}

	client.StartSubscription(onMessageReceived, filter)
	// client.Presence(onMessageReceived, *topic)
	wg.Done()
	wg.Wait()

	time.Sleep(5 * time.Second)

	for i := 0; i < pubCnt; i++ {
		<- client.Publish(MQTT.QoS(qos), *topic, []byte(*message))
		log.Printf("Published\n")
		time.Sleep(time.Duration(interval) * time.Millisecond)
	}
}

func main() {
	log.SetPrefix("++++++")
	appkey := flag.String("appkey", "563c4afef085fc471efdf803", "YunBa appkey")
	topic := flag.String("topic", "topic_test", "Topic to publish the messages on")
	message := flag.String("message", "hello", "Message to be published")
	qos := flag.Int("qos", 0, "The QoS to send the messages at")
	broker := flag.String("broker", "tcp://123.56.125.40:1883", "Broker address, default: tcp://123.56.125.40:1883")

	client := flag.Int("client", 1, "Number of clients")
	ecahPubCnt := flag.Int("each", 1, "Each client publish count")
	interval := flag.Int("interval", 1000, "Publish interval for a routine, millisecond")

	reg := flag.Bool("reg", false, "If register and save the returned infomation")
	file := flag.String("file", "./reg.info", "Register infomation file")
	//retained := flag.Bool("retained", false, "Are the messages sent with the retained flag")
	flag.Parse()

	if *reg == true {
		regFile, err := os.Create(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer regFile.Close()
		for i := 0; i < *client; i++ {
			wg.Add(1)
			go doReg(i, regFile, appkey, topic, *qos, broker)
			time.Sleep(10 * time.Millisecond)
		}
		wg.Wait()
		// regFile.Sync()
	} else {

		regFile, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer regFile.Close()
		fileScanner := bufio.NewScanner(regFile)

		cnt := 0
		for fileScanner.Scan() {
			
			regInfo := strings.Split(fileScanner.Text(), "|")
			wg.Add(1)
			go doWork(cnt, &regInfo[0], &regInfo[1], &regInfo[2], broker, topic, *qos, message, *ecahPubCnt, *interval)
			time.Sleep(10 * time.Millisecond)
			cnt++
		}

		msgNeedRecv := (cnt * *ecahPubCnt * cnt)

		for {
			time.Sleep(2 * time.Second)
			if msgRecv == msgNeedRecv {
				break
			}
			log.Printf("msgRecv: %d\n", msgRecv)
		}
		log.Printf("msgRecv: %d\n", msgRecv)
	}
}
