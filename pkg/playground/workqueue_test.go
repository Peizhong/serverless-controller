package playground

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

func TestLimiter(t *testing.T) {
	lim := rate.NewLimiter(rate.Limit(2), 10)
	t.Log(lim.Limit(), lim.Burst())
	var wg sync.WaitGroup
	wg.Add(20)
	for i := 0; i < 20; i++ {
		idx := i
		go func() {
			defer wg.Done()
			if !lim.Allow() {
				t.Log("not allow", idx)
			}
		}()
	}
	wg.Wait()
}

func TestWorkQueue(t *testing.T) {
	// 默认的限速器控制器，包含了2种限速规则，选择最大的
	// 1. 错误越多，限速时间越长
	// 2. 基于并发量
	// 限速器计算出要多久才能加入队列
	limiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(2), 2)},
	)
	q := workqueue.NewNamedRateLimitingQueue(limiter, "Foos")
	go func() {
		for i := 0; i < 20; i++ {
			// 根据ratelimit的时间调整，额外的定时器控制，到达之间后才加入到queue。newDelayingQueue长度1000
			q.AddRateLimited(struct {
				Id int
			}{
				Id: i,
			})
		}
	}()
	go func() {
		for {
			// get的数据不受管控，只要队列有数据，就能拿得到。是在AddRateLimited的时候控制的
			v, shutdown := q.Get()
			if shutdown {
				t.Log("shutdown")
				break
			}
			t.Log(time.Now().Unix(), v)
			q.Forget(v)
			q.Done(v)

		}
	}()
	<-time.After(time.Second * 10)
}

func TestAddQueue(t *testing.T) {
	var a interface{} = struct {
		A int
		B int
	}{}
	var b interface{} = map[string]interface{}{}
	if a == b {
		t.Log("same")
	}
	t.Log(reflect.TypeOf(b).Comparable())
	c := map[string]interface{}{}
	t.Log(reflect.TypeOf(c).Comparable())
}
