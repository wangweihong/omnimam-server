package waitgroup_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/waitgroup"
)

func TestGenerictWaitGroup(t *testing.T) {
	Convey("TestGenerictWaitGroup", t, func() {
		Convey("正常任务完成", func() {
			baseCtx := context.Background()
			group := waitgroup.NewGenericGroup[string](baseCtx)
			group.Start(waitgroup.NewGenericFunc("fun1", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(50 * time.Millisecond)
				return waitgroup.NewGenericResult("success", nil)
			}))
			group.Wait()
			So(group.GetResults()["fun1"].Data, ShouldEqual, "success")
		})

		Convey("单任务超时", func() {
			baseCtx := context.Background()
			group := waitgroup.NewGenericGroup[string](baseCtx)

			group.Start(waitgroup.NewGenericFunc("funTimeout", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(100 * time.Millisecond)
				// 这步应该不处理
				return waitgroup.NewGenericResult("fail", nil)
			}), 50*time.Millisecond)
			group.Start(waitgroup.NewGenericFunc("funOK", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(50 * time.Millisecond)
				return waitgroup.NewGenericResult("success", nil)
			}))
			group.Wait()
			fmt.Println(group.GetResults())
			So(group.GetResults()["funTimeout"].Error, ShouldNotBeNil)
			So(group.GetResults()["funOK"].Error, ShouldBeNil)
		})

		Convey("全局超时", func() {
			baseCtx := context.Background()
			group := waitgroup.NewGenericGroup[string](baseCtx)

			group.WithGlobalTimeout(50 * time.Millisecond)

			group.Start(waitgroup.NewGenericFunc("funTimeout", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(100 * time.Millisecond)
				// 这步应该不处理
				return waitgroup.NewGenericResult("fail", nil)
			}))
			group.Start(waitgroup.NewGenericFunc("funOK", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(50 * time.Millisecond)
				return waitgroup.NewGenericResult("success", nil)
			}))
			group.Start(waitgroup.NewGenericFunc("funTimeoutOverride", func(ctx context.Context) waitgroup.GenericResult[string] {
				time.Sleep(100 * time.Millisecond)
				return waitgroup.NewGenericResult("success", nil)
			}), 150*time.Millisecond)
			group.Wait()
			fmt.Println(group.GetResults())
			So(group.GetResults()["funTimeout"].Error, ShouldNotBeNil)
			So(group.GetResults()["funOK"].Error, ShouldBeNil)
			So(group.GetResults()["funTimeoutOverride"].Error, ShouldBeNil)
		})

		Convey("全局取消", func() {
			baseCtx, cancel := context.WithCancel(context.Background())
			go func() {
				select {
				case <-time.After(50 * time.Millisecond):
					cancel()
				}
			}()
			group := waitgroup.NewGenericGroup[string](baseCtx)

			group.Start(waitgroup.NewGenericFunc("funCancel", func(context.Context) waitgroup.GenericResult[string] {
				time.Sleep(100 * time.Millisecond)
				return waitgroup.NewGenericResult("fail", nil)
			}))

			group.Wait()
			fmt.Println(group.GetResults())
			So(group.GetResults()["funCancel"].Error, ShouldNotBeNil)
		})
	})
}
