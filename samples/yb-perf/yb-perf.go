package main

import (
	"os"
	"bufio"
	"log"
	"flag"
	"time"
	"math"
	"strconv"
	"sync"
	"strings"
	"encoding/binary"
	MQTT "../../"
	"os/signal"
)

var msgRecv int = 0
var wgReg sync.WaitGroup
var wgSub sync.WaitGroup

var pubTimes [][]int64 // ns
var usedTimes [][]int64 // ms

var minTime int64 = math.MaxInt64
var maxTime int64 = math.MinInt64
var lockTime sync.Mutex

var beginTimeSet bool = false
var lockTime2 sync.Mutex
var pubStarted bool = false
var connectedCnt int = 0
var subacked int = 0
var connectionLost int = 0
var beginTime int64 = 0
var endTime int64 = 0

func OnConnectionLost(client *MQTT.MqttClient, reason error) {
	connectionLost++
	log.Printf("error: %v\n", reason)
}

func onSuback() {
	subacked++
}

func defaultPublishHandler(client *MQTT.MqttClient, msg MQTT.Message) {
	//log.Printf("topic: %s\n", msg.Topic())
	//log.Printf("msg: %s\n", msg.Payload())
}

func onMessageReceived(client *MQTT.MqttClient, message MQTT.Message) {
	if !pubStarted {
		return
	}
	ns := time.Now().UnixNano()
	data := message.Payload()

	//log.Printf("recv msg len: %d", len(data))

	i := binary.LittleEndian.Uint32(data)
	j := binary.LittleEndian.Uint32(data[4:])

	ms := (ns - pubTimes[i][j]) / 1000000
	usedTimes[i][j] += ms

	lockTime.Lock()
	if minTime > ms {
		minTime = ms
	}
	if maxTime < ms {
		maxTime = ms
	}
	lockTime.Unlock()

	lockTime2.Lock()
	endTime = time.Now().UnixNano() / 1000000
	lockTime2.Unlock()

	// log.Printf("recv msg len: %d", len(data))

	// log.Printf("Received message on topic: %s\n", message.Topic())
	// data := message.Payload()
	// l := len(data)
	// if l > 8 {
	// 	l = 8;
	// }
	// log.Printf("message: %s\n", data[:l])
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

	line := strings.Join([]string{regInfo.Client, regInfo.UserName, regInfo.Password}, "|")

	log.Printf("line: %s\n", line)
	writer := bufio.NewWriter(regFile)
	_, err = writer.WriteString(line + "\n")
	if err != nil {
		log.Fatal(err)
	}

	writer.Flush()
	wgReg.Done()
}

func doTest(index int, clientid *string, user *string, pass *string, broker *string, topic *string, qos int, msgLen int, pubEach int, interval int, mode int) {
	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(*broker)
	connOpts.SetClientId(*clientid)
	connOpts.SetCleanSession(true)
	connOpts.SetProtocolVersion(0x13)

	connOpts.SetUsername(*user)
	connOpts.SetPassword(*pass)

	connOpts.SetDefaultPublishHandler(defaultPublishHandler)
	// connOpts.SetKeepAlive(300)
	connOpts.SetOnConnectionLost(OnConnectionLost)

	client := MQTT.NewClient(connOpts)
	_, err := client.Start()
	if err != nil {
		panic(err)
	} else {
		connectedCnt++
	}

	filter, err := MQTT.NewTopicFilter(*topic, byte(qos))
	if err != nil {
		log.Fatal(err)
	}

	if mode == 1 {
		// sub
		client.StartSubscription(onMessageReceived, onSuback, filter)
	} else if mode == 2 {
		// pub
		msg := make([]byte, msgLen)
		binary.LittleEndian.PutUint32(msg, uint32(index))

		wgSub.Wait()

		time.Sleep(5 * time.Second)
		pubStarted = true

		if !beginTimeSet {
			lockTime2.Lock()
			if !beginTimeSet {
				beginTimeSet = true
				beginTime = time.Now().UnixNano() / 1000000
			}
			lockTime2.Unlock()
		}

		for i := 0; i < pubEach; i++ {
			binary.LittleEndian.PutUint32(msg[4:], uint32(i))
			pubTimes[index][i] = time.Now().UnixNano()
			<-client.Publish(MQTT.QoS(qos), *topic, msg)
			log.Printf("published\n")
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
	}
	// unsub
	// client.EndSubscription(*topic)
	// wgWork.Done()
}

func main() {
	appkey := flag.String("appkey", "563c4afef085fc471efdf803", "YunBa appkey")
	topic := flag.String("topic", "topic_test", "Topic to publish the messages on")
	msgLen := flag.Int("msglen", 8, "Length of message to be published, at least 8 bytes for statistics infomation")
	qos := flag.Int("qos", 0, "The QoS to send the messages at")
	broker := flag.String("broker", "tcp://123.56.125.40:1883", "Broker address, default: tcp://123.56.125.40:1883")

	reg := flag.Int("reg", 0, "Number of registration")
	pubCnt := flag.Int("pubcnt", 1, "Number of client for publish")
	subCnt := flag.Int("subcnt", 1, "Number of client for subscribe")
	pubEach := flag.Int("pubeach", 1, "How many publish one client do")
	interval := flag.Int("interval", 1000, "Interval of publishes(when [pubeach] > 1), millisecond")

	// reg := flag.Bool("reg", false, "Only register and save the infomation")
	file := flag.String("file", "./reg.info", "Register infomation file")
	//retained := flag.Bool("retained", false, "Are the messages sent with the retained flag")
	flag.Parse()

	if *reg > 0 {
		regFile, err := os.Create(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer regFile.Close()
		for i := 0; i < *reg; i++ {
			wgReg.Add(1)
			go doReg(i, regFile, appkey, topic, *qos, broker)
			time.Sleep(10 * time.Millisecond)
		}
		wgReg.Wait()
		// regFile.Sync()
	} else {
		if *msgLen < 8 {
			*msgLen = 8
		}
		pubTimes = make([][]int64, *pubCnt)
		for i := range pubTimes {
			pubTimes[i] = make([]int64, *pubEach)
		}
		usedTimes = make([][]int64, *pubCnt)
		for i := range usedTimes {
			usedTimes[i] = make([]int64, *pubEach)
		}

		regFile, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer regFile.Close()
		fileScanner := bufio.NewScanner(regFile)

		wgSub.Add(1)
		index := 0
		for fileScanner.Scan() {
			log.Printf("add sub[%d]: %s\n", index, fileScanner.Text())
			regInfo := strings.Split(fileScanner.Text(), "|")
			go doTest(index, &regInfo[0], &regInfo[1], &regInfo[2], broker, topic, *qos, *msgLen, *pubEach, *interval, 1)
			time.Sleep(120 * time.Millisecond)
			index++
			if index == *subCnt {
				break
			}
		}
		if index < *subCnt {
			log.Printf("no enough reg info for sub\n")
			return
		}

		index = 0
		for fileScanner.Scan() {
			log.Printf("add pub[%d]: %s\n", index, fileScanner.Text())
			regInfo := strings.Split(fileScanner.Text(), "|")
			go doTest(index, &regInfo[0], &regInfo[1], &regInfo[2], broker, topic, *qos, *msgLen, *pubEach, *interval, 2)
			time.Sleep(10 * time.Millisecond)
			index++
			if index == *pubCnt {
				break
			}
		}
		if index < *pubCnt {
			log.Printf("no enough reg info for pub\n")
			return
		}

		stop := false
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			stop = true
		}()

		clientCnt := (*subCnt + *pubCnt)
		// wait connecet
		waitCnt := clientCnt / 100
		log.Printf("wait connect %d seconds...\n", waitCnt)
		for {
			time.Sleep(1 * time.Second)
			log.Printf("connected: %d\n", connectedCnt)
			if stop || connectedCnt == clientCnt || waitCnt == 0 {
				break
			}
			waitCnt--
		}

		// wait sub
		waitCnt = *subCnt / 50
		log.Printf("wait sub %d seconds...\n", waitCnt)
		for {
			time.Sleep(1 * time.Second)
			log.Printf("subacked: %d\n", subacked)
			if stop || subacked == *subCnt || waitCnt == 0 {
				break
			}
			waitCnt--
		}
		wgSub.Done()

		pubTotal := *pubEach * *pubCnt
		msgTotal := pubTotal * *subCnt
		// wait message
		waitCnt = 20 + pubTotal / 10
		log.Printf("wait message %d seconds...\n", waitCnt)
		for {
			time.Sleep(1 * time.Second)
			if pubStarted {
				log.Printf("received: %d\n", msgRecv)
			} else {
				continue
			}
			if stop || msgRecv == msgTotal || waitCnt == 0 {
				break
			}
			waitCnt--
		}

		log.Printf("\n")
		log.Printf("pub client: %d, sub client: %d\n", *pubCnt, *subCnt)
		log.Printf("connection lost: %d\n", connectionLost)
		log.Printf("\n")
		log.Printf("connect: expec/succ/fail = %d/%d/%d\n", clientCnt, connectedCnt, clientCnt - connectedCnt)
		log.Printf("sub: expec/succ/fail = %d/%d/%d\n", *subCnt, subacked, *subCnt - subacked)

		if msgRecv == 0 {
			log.Printf("\n")
			log.Printf("no message received\n")
			log.Printf("\n")
		} else {
			totalTime := int64(0)

			for _, usedTime := range usedTimes {
				for _, ms := range usedTime {
					totalTime += ms
				}
			}
			log.Printf("message: expec/succ/fail = %d/%d/%d\n", msgTotal, msgRecv, msgTotal - msgRecv)
			log.Printf("time: max/min/avg = %d/%d/%d\n", maxTime, minTime, totalTime / int64(msgRecv))
			log.Printf("\n")
			if msgRecv != msgTotal {
				log.Printf("some message lost\n")
				log.Printf("\n")
			}
			log.Printf("for excel: %d\t%d\t%d\t%d\n", maxTime, minTime, totalTime / int64(msgRecv), msgTotal - msgRecv)
			log.Printf("\n")

			//log.Printf("\n")
			//log.Printf("pub: %d, sub: %d, received: %d, lost: %d", pubTotal, subClient, msgRecv, msgTotal - msgRecv)
			//log.Printf("serial: %d ms, parallel: %d ms, max: %d ms, min: %d ms, avg: %d ms\n", totalTime, endTime - beginTime, maxTime, minTime, totalTime / int64(msgRecv))
			//log.Printf("%d/%d/%d\n", maxTime, minTime, totalTime / int64(subClient * *pubEach * *pubClient))
			//log.Printf("\n")
		}
	}
}
