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
	MQTT "github.com/shdxiang/yb-perf-go"
	"os/signal"
)

const (
	MODE_SUB = 1
	MODE_PUB = 2
	MODE_SUB_ONLY = 3
	MODE_PUB_ONLY = 4
)

var msgRecv int = 0
var msgPub int = 0
var wgReg sync.WaitGroup
var wgSub sync.WaitGroup
var wgUnsub sync.WaitGroup
var wgExit sync.WaitGroup

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

func defaultPublishHandler(client *MQTT.MqttClient, message MQTT.Message) {
	//log.Printf("defaultPublishHandler\n")
}

func onMessageReceivedStat(client *MQTT.MqttClient, message MQTT.Message) {
	ns := time.Now().UnixNano()
	data := message.Payload()
	log.Printf("received: index: %d, topic: %s, len: %d, time: %d", msgRecv, message.Topic(), len(data), ns / 1000000)
	msgRecv++
	if !pubStarted {
		log.Printf("received message before publish")
		return
	}

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
}

func onMessageReceivedDemon(client *MQTT.MqttClient, message MQTT.Message) {
	ns := time.Now().UnixNano()
	data := message.Payload()
	log.Printf("received: index: %d, topic: %s, len: %d, time: %d", msgRecv, message.Topic(), len(data), ns / 1000000)
	msgRecv++
}

func doRegister(index int, regFile *os.File, appkey *string, topic *string, qos int, broker *string) {
	deviceId := strconv.Itoa(time.Now().Second()) + strconv.Itoa(index)

	yunbaClient := &MQTT.YunbaClient{*appkey, deviceId}
	regInfo, err := yunbaClient.Reg()
	if err != nil {
		log.Fatal(err)
	}

	if regInfo.ErrCode != 0 {
		log.Fatal("error:", regInfo.ErrCode)
	}

	line := strings.Join([]string{regInfo.Client, regInfo.UserName, regInfo.Password}, "|")

	log.Printf("reg info: %s\n", line)
	writer := bufio.NewWriter(regFile)
	_, err = writer.WriteString(line + "\n")
	if err != nil {
		log.Fatal(err)
	}

	writer.Flush()
	wgReg.Done()
}

func register(file *string, count int, appkey *string, topic *string, qos int, broker *string) {
	regFile, err := os.Create(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer regFile.Close()
	for i := 0; i < count; i++ {
		wgReg.Add(1)
		go doRegister(i, regFile, appkey, topic, qos, broker)
		time.Sleep(10 * time.Millisecond)
	}
	wgReg.Wait()
	regFile.Sync()
}

func test(index int, clientid *string, user *string, pass *string, broker *string, topic *string, qos int, msgLen int, pubEach int, interval int, mode int) {
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

	if mode == MODE_SUB {
		// sub
		client.StartSubscription(onMessageReceivedStat, onSuback, filter)
		//wgSub.Done()
		wgUnsub.Add(1)

		wgExit.Wait()
		// unsub
		client.EndSubscription(*topic)
		//time.Sleep(2 * time.Second)
		wgUnsub.Done()
	} else if mode == MODE_PUB {
		// pub
		msg := make([]byte, msgLen)
		binary.LittleEndian.PutUint32(msg, uint32(index))

		wgSub.Wait()
		pubStarted = true
		time.Sleep(1 * time.Second)

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
			log.Printf("published: index: %d:%d, topic: %s, len: %d, time: %d\n", index, i, *topic, msgLen, time.Now().UnixNano() / 1000000)
			msgPub++
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
	} else if mode == MODE_SUB_ONLY {
		// sub
		client.StartSubscription(onMessageReceivedDemon, onSuback, filter)
		//wgSub.Done()
		wgUnsub.Add(1)

		wgExit.Wait()
		// unsub
		client.EndSubscription(*topic)
		//time.Sleep(2 * time.Second)
		wgUnsub.Done()
	} else if mode == MODE_PUB_ONLY {
		// pub
		msg := make([]byte, msgLen)
		binary.LittleEndian.PutUint32(msg, uint32(index))

		wgSub.Wait()
		pubStarted = true
		time.Sleep(1 * time.Second)

		for i := 0; i < pubEach; i++ {
			binary.LittleEndian.PutUint32(msg[4:], uint32(i))
			<-client.Publish(MQTT.QoS(qos), *topic, msg)
			log.Printf("published: index: %d:%d, topic: %s, len: %d, time: %d\n", index, i, *topic, msgLen, time.Now().UnixNano() / 1000000)
			msgPub++
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
	}
}

func initStat(pubCnt int, pubEach int) {
	pubTimes = make([][]int64, pubCnt)
	for i := range pubTimes {
		pubTimes[i] = make([]int64, pubEach)
	}
	usedTimes = make([][]int64, pubCnt)
	for i := range usedTimes {
		usedTimes[i] = make([]int64, pubEach)
	}
}

func addTest(fileScanner *bufio.Scanner, count int, broker *string, topic *string, qos int, msgLen int, pubEach int, interval int, mode int) {
	index := 0
	for fileScanner.Scan() {
		if index >= count {
			break
		}
		log.Printf("addTest[%d], mode: %d, info: %s\n", index, mode, fileScanner.Text())
		regInfo := strings.Split(fileScanner.Text(), "|")
		go test(index, &regInfo[0], &regInfo[1], &regInfo[2], broker, topic, qos, msgLen, pubEach, interval, mode)
		time.Sleep(120 * time.Millisecond)
		index++
	}
	if index < count {
		log.Fatal("no enough reg info for test\n")
	}
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

	timeout := flag.Int("timeout", 0, "The time we wait for message, second, 0: auto compute, -1: forerver")
	//daemon := flag.Bool("daemon", false, "Only subscribe and receive messages, pubCnt will be ignored")

	file := flag.String("file", "./reg.info", "Register infomation file")
	//retained := flag.Bool("retained", false, "Are the messages sent with the retained flag")
	flag.Parse()

	wgExit.Add(1)
	if *reg > 0 {
		register(file, *reg, appkey, topic, *qos, broker)
	} else {
		if *pubCnt > 0 && *subCnt > 0 {
			initStat(*pubCnt, *pubEach)
			if *msgLen < 8 {
				*msgLen = 8
			}
		}

		regFile, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer regFile.Close()
		fileScanner := bufio.NewScanner(regFile)

		// sub
		mode := MODE_SUB
		if *pubCnt <= 0 {
			mode = MODE_SUB_ONLY
		}
		wgSub.Add(1)
		addTest(fileScanner, *subCnt, broker, topic, *qos, *msgLen, *pubEach, *interval, mode)

		// pub
		mode = MODE_PUB
		if *subCnt <= 0 {
			mode = MODE_PUB_ONLY
		}
		addTest(fileScanner, *pubCnt, broker, topic, *qos, *msgLen, *pubEach, *interval, mode)

		stop := false
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			<-c
			stop = true
		}()

		clientCnt := (*subCnt + *pubCnt)
		// wait connect
		waitCnt := 10 + clientCnt / 100
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
		waitCnt = 10 + *subCnt / 50
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
		if *timeout == 0 {
			waitCnt = 5 * (10 + pubTotal / 10)
		} else {
			waitCnt = *timeout
		}

		if *pubCnt == 0 || waitCnt == -1 {
			log.Printf("wait message forever...\n")
		} else {
			log.Printf("wait message %d seconds...\n", waitCnt)
		}

		for {
			if stop {
				break
			}
			time.Sleep(200 * time.Millisecond)

			if *pubCnt == 0 || waitCnt == -1 {
				continue
			}
			if (msgRecv == msgTotal || waitCnt == 0) && (msgPub == pubTotal) {
				break
			}
			waitCnt--
		}

		log.Printf("prepare exiting...\n")
		wgExit.Done()
		wgUnsub.Wait()

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
		} else if *pubCnt == 0 {
			log.Printf("\n")
			log.Printf("message receive: %d\n", msgRecv)
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
		}
	}
}
