package nats

//
//import (
//	"bytetrade.io/web3os/fs-lib/jfsnotify"
//	"fmt"
//	"k8s.io/klog/v2"
//	"path/filepath"
//	"sync"
//	"time"
//)
//
//type EventWrapper struct {
//	Event jfsnotify.Event
//	Time  time.Time
//}
//
//var (
//	eventQueue    = make(chan EventWrapper, 100)
//	lastEvent     EventWrapper
//	eventMutex    sync.Mutex
//	lastEventType jfsnotify.Op
//	ticker        *time.Ticker
//	timeout       = 1 * time.Second
//	emptyEvent    = EventWrapper{Event: jfsnotify.Event{}}
//)
//
//func AddEventToQueue(e jfsnotify.Event, immediate bool) {
//	if immediate {
//		klog.Infoln("event Queue immediately send event: ", e.Name, " at time ", time.Now())
//		sendEvent(EventWrapper{Event: e, Time: time.Now()})
//		resetTimer()
//	} else {
//		klog.Infoln("event Queue get event: ", e.Name, " at time ", time.Now())
//		eventQueue <- EventWrapper{Event: e, Time: time.Now()}
//	}
//}
//
//func checkEventQueue() {
//	for {
//		select {
//		case ew := <-eventQueue:
//			eventMutex.Lock()
//			if filepath.Dir(ew.Event.Name) != filepath.Dir(lastEvent.Event.Name) { // || !ew.Event.Has(lastEventType) {
//				klog.Infoln("deal with a new event: ", ew.Event.Name, " at time ", time.Now())
//				if lastEvent.Event != emptyEvent.Event {
//					sendEvent(lastEvent)
//				}
//				lastEvent = ew
//				lastEventType = ew.Event.Op
//			} else {
//				lastEvent = ew
//			}
//			eventMutex.Unlock()
//			resetTimer()
//		case <-ticker.C:
//			eventMutex.Lock()
//			if lastEvent.Event != emptyEvent.Event {
//				klog.Infoln("deal with a timed event:", lastEvent.Event.Name, " at time ", time.Now())
//				sendEvent(lastEvent)
//				lastEvent = emptyEvent
//			}
//			eventMutex.Unlock()
//		}
//	}
//}
//
//func sendEvent(ew EventWrapper) {
//	//msg := fmt.Sprintf("%s event in the directory: %s", ew.Event.Op, filepath.Dir(ew.Event.Name))
//	msg := filepath.Dir(ew.Event.Name)
//	SendMessage(msg)
//}
//
//func resetTimer() {
//	//ticker.Stop()
//	//ticker = time.NewTicker(timeout)
//	ticker.Reset(timeout)
//}
