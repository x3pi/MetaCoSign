package listener

import (
	"log"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/event"
	"github.com/meta-node-blockchain/meta-node/test/contract"
)

type EventListener struct {
	fileContract *contract.FileContract
}

func NewEventListener(fileContract *contract.FileContract) *EventListener {
	return &EventListener{
		fileContract: fileContract,
	}

}
func (e *EventListener) Start() {
	go e.ListenForAdmin1Events() // Tên cũ của bạn
}
func (e *EventListener) subscribeWithRetry(logs chan<- *contract.FileContractPing) event.Subscription {
	for {
		
		sub, err := e.fileContract.WatchPing(&bind.WatchOpts{}, logs, nil)
		if err != nil {
			log.Printf("Lỗi đăng ký sự kiện: %v. Sẽ thử lại sau %v.", err, 1*time.Second)
			time.Sleep(1 * time.Second)
			continue // Thử lại vòng lặp
		}
		log.Println(">>> Đăng ký sự kiện thành công! Đang chờ thông báo...")
		return sub // Trả về subscription thành công
	}
}
func (e *EventListener) ListenForAdmin1Events() {
	logs := make(chan *contract.FileContractPing)
	for {
		sub := e.subscribeWithRetry(logs)
	eventLoop:
		for {
			select {
			case err := <-sub.Err():
				if err != nil {
					log.Printf("Lỗi trong subscription: %v. Đang cố gắng đăng ký lại...", err)
					sub.Unsubscribe()
				}
				break eventLoop // Thoát vòng lặp nếu có lỗi
			case eventLog := <-logs:
				log.Printf("✅ Sự kiện FileActivated nhận được: FileID=%s, Activator=%s", eventLog.Id, eventLog.Message)
			}
		}
	}
}
