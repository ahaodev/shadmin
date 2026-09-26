package captcha

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"shadmin/internal/cacher"
)

// rssKB 读 /proc/self/statm 返回进程 RSS（KB）；非 Linux 或读取失败返回 -1。
func rssKB() int64 {
	f, err := os.Open("/proc/self/statm")
	if err != nil {
		return -1
	}
	defer f.Close()
	line, _ := bufio.NewReader(f).ReadString('\n')
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return -1
	}
	v, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return -1
	}
	return v * (int64(os.Getpagesize()) / 1024)
}

// heapAfterGC 触发两轮 GC 后返回存活堆字节数（≈ 真实滞留量）。
func heapAfterGC() uint64 {
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // 给 finalizer/ scavenger 一点时间
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapInuse
}

// countCaptchaItems 统计 captcha 命名空间下逻辑存活的 challenge 条数。
// 用 Exists（Get 的惰性过期检查）而非裸条目数：go-cache Items() 会包含
// 已过期但 janitor 尚未清理的条目。
func countCaptchaItems(t *testing.T, c cacher.Cacher, ctx context.Context) int {
	t.Helper()
	n := 0
	if err := c.Iterator(ctx, captchaNS, func(ctx context.Context, key, value string) bool {
		alive, err := c.Exists(ctx, captchaNS, key)
		if err != nil {
			t.Fatalf("exists %q: %v", key, err)
		}
		if alive {
			n++
		}
		return true
	}); err != nil {
		t.Fatalf("iterate captcha namespace: %v", err)
	}
	return n
}

// TestVerifyFailureReclaimsChallenge 定量验证：校验失败路径上 challenge 记录
// 是否被正常消费/过期回收，堆内存是否收敛而非单调增长。
func TestVerifyFailureReclaimsChallenge(t *testing.T) {
	c := cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute})
	m, err := NewSlideManager(c)
	if err != nil {
		t.Fatalf("new slide manager: %v", err)
	}
	ctx := context.Background()

	baseline := heapAfterGC()
	t.Logf("baseline: heap=%d KB, rss=%d KB", baseline/1024, rssKB())

	// 阶段 1：模拟连续失败——每个 challenge 生成后连错 3 次（次数耗尽）。
	const cycles = 300
	var responseBytes uint64
	for i := range cycles {
		ch, err := m.Generate(ctx, "")
		if err != nil {
			t.Fatalf("generate %d: %v", i, err)
		}
		responseBytes += uint64(len(ch.MasterImage) + len(ch.TileImage))
		// y=9999 必然越界，保证校验失败
		for j := 0; j < m.maxAttempt; j++ {
			if err := m.Verify(ctx, ch.CaptchaID, 0, 9999); err == nil {
				t.Fatalf("cycle %d attempt %d: expected failure", i, j)
			}
		}
	}
	afterCycles := heapAfterGC()
	items := countCaptchaItems(t, c, ctx)
	t.Logf("after %d failure cycles: heap=%d KB, rss=%d KB, captcha items=%d, image payload churn=%d KB",
		cycles, afterCycles/1024, rssKB(), items, responseBytes/1024)

	if items != 0 {
		t.Errorf("challenges left after exhausted attempts: %d, want 0", items)
	}
	if afterCycles > baseline*2 {
		t.Errorf("heap grew monotonically: baseline=%d KB, after=%d KB", baseline/1024, afterCycles/1024)
	}

	// 阶段 2：失败 1 次后写回的 challenge 应随剩余 TTL 过期，而不是永久滞留。
	ch, err := m.Generate(ctx, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := m.Verify(ctx, ch.CaptchaID, 0, 9999); err == nil {
		t.Fatal("expected failure")
	}
	if got := countCaptchaItems(t, c, ctx); got != 1 {
		t.Fatalf("items after 1 failure = %d, want 1 (written back with remaining TTL)", got)
	}

	time.Sleep(m.ttl + 5*time.Second)
	if got := countCaptchaItems(t, c, ctx); got != 0 {
		t.Errorf("items after TTL elapsed = %d, want 0", got)
	}
	idle := heapAfterGC()
	t.Logf("idle after TTL: heap=%d KB, rss=%d KB", idle/1024, rssKB())
	if idle > baseline*2 {
		t.Errorf("idle heap not converged: baseline=%d KB, idle=%d KB", baseline/1024, idle/1024)
	}
}

// TestGenerateRefreshReleasesOldChallenge 验证带 oldID 的刷新会主动回收旧 challenge。
func TestGenerateRefreshReleasesOldChallenge(t *testing.T) {
	c := cacher.NewMemoryCache(cacher.MemoryConfig{CleanupInterval: time.Minute})
	m, err := NewSlideManager(c)
	if err != nil {
		t.Fatalf("new slide manager: %v", err)
	}
	ctx := context.Background()

	first, err := m.Generate(ctx, "")
	if err != nil {
		t.Fatalf("generate first: %v", err)
	}
	second, err := m.Generate(ctx, first.CaptchaID)
	if err != nil {
		t.Fatalf("generate refresh: %v", err)
	}
	if got := countCaptchaItems(t, c, ctx); got != 1 {
		t.Fatalf("items after refresh = %d, want 1 (old must be released)", got)
	}
	m.Invalidate(ctx, second.CaptchaID)
	if got := countCaptchaItems(t, c, ctx); got != 0 {
		t.Fatalf("items after invalidate = %d, want 0", got)
	}
}
