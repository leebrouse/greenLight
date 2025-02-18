package main

import (
	"context" // New import
	"errors"  // New import
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	// 创建一个 HTTP 服务器实例
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.router(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 创建一个 shutdownError 通道，用于接收 Shutdown() 返回的错误
	shutdownError := make(chan error)

	// 启动一个 goroutine 来等待停止信号并执行优雅关机
	go func() {
		// 监听中断信号（SIGINT 或 SIGTERM）
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit
		// 记录日志，显示接收到的信号
		app.logger.PrintInfo("shutting down server", map[string]string{
			"signal": s.String(),
		})

		// 创建一个带有 5 秒超时的 context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 调用服务器的 Shutdown 方法，传递 context 来控制超时
		shutdownError <- srv.Shutdown(ctx)
	}()

	// 记录日志，显示服务器启动信息
	app.logger.PrintInfo("starting server", map[string]string{
		"addr": fmt.Sprintf(":%d", app.config.port),
		"env":  app.config.env,
	})

	// 启动服务器并监听请求
	err := srv.ListenAndServe()
	// 如果错误不是 http.ErrServerClosed，返回该错误
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// 等待来自 shutdownError 通道的返回值，表示 Shutdown 是否成功
	err = <-shutdownError
	if err != nil {
		return err
	}

	// 记录日志，显示服务器已停止
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": fmt.Sprintf(":%d", app.config.port),
	})

	return nil
}
