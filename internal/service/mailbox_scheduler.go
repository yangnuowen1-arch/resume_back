package service

import (
	"context"
	"log"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

// MailboxScheduler 定时触发所有邮箱账号的扫描任务。
type MailboxScheduler struct {
	mailboxService MailboxService
	accountRepo    repository.MailboxAccountRepository
	cronHour       int // 每天几点触发（0-23）
	stopCh         chan struct{}
}

// NewMailboxScheduler 创建定时扫描器。cronHour 为每天触发的小时（0-23）。
func NewMailboxScheduler(mailboxService MailboxService, accountRepo repository.MailboxAccountRepository, cronHour int) *MailboxScheduler {
	if cronHour < 0 || cronHour > 23 {
		cronHour = 23 // 默认 23:00
	}
	return &MailboxScheduler{
		mailboxService: mailboxService,
		accountRepo:    accountRepo,
		cronHour:       cronHour,
		stopCh:         make(chan struct{}),
	}
}

// Start 启动定时器，在单独的 goroutine 中运行。
func (s *MailboxScheduler) Start() {
	go s.run()
	log.Printf("mailbox scheduler started cronHour=%d", s.cronHour)
}

// Stop 停止定时器。
func (s *MailboxScheduler) Stop() {
	close(s.stopCh)
	log.Println("mailbox scheduler stopped")
}

func (s *MailboxScheduler) run() {
	ticker := time.NewTicker(s.nextTickDuration())
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.triggerScheduledScan()
			// 重置 ticker 到下一个触发时间点
			ticker.Reset(s.nextTickDuration())
		}
	}
}

// nextTickDuration 计算距离下一次触发点的时长（今天或明天的指定小时）。
func (s *MailboxScheduler) nextTickDuration() time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), s.cronHour, 0, 0, 0, now.Location())

	// 如果今天的触发点已过，推到明天
	if next.Before(now) || next.Equal(now) {
		next = next.Add(24 * time.Hour)
	}

	duration := next.Sub(now)
	log.Printf("mailbox scheduler next trigger at %s (in %s)", next.Format("2006-01-02 15:04:05"), duration)
	return duration
}

// triggerScheduledScan 遍历所有已连接的邮箱账号，为每个账号入队一个扫描任务。
func (s *MailboxScheduler) triggerScheduledScan() {
	ctx := context.Background()
	start := time.Now()
	log.Println("mailbox scheduler triggered scheduled scan")

	accounts, err := s.accountRepo.List(ctx)
	if err != nil {
		log.Printf("mailbox scheduler list accounts failed error=%v", err)
		return
	}

	enqueued := 0
	for _, account := range accounts {
		taskID, err := s.mailboxService.EnqueueScan(ctx, account.ID, ScanTriggerScheduled)
		if err != nil {
			log.Printf("mailbox scheduler enqueue scan failed accountId=%d error=%v", account.ID, err)
			continue
		}
		log.Printf("mailbox scheduler enqueued scan taskId=%d accountId=%d", taskID, account.ID)
		enqueued++
	}

	log.Printf("mailbox scheduler finished scheduled scan accounts=%d enqueued=%d duration=%s",
		len(accounts), enqueued, time.Since(start))
}
