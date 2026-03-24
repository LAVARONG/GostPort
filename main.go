package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

//go:embed web/*
var webFS embed.FS

var (
	port = flag.String("port", "8080", "管理面板启动端口")
)

func main() {
	flag.Parse()

	log.Println("=== GostPort 极简端口映射系统 ===")
	log.Println("设计理念：极致留白 & 单文件部署")

	// 0. 程序启动时预检查/自机下载底层依赖
	if _, err := EnsureGost(); err != nil {
		log.Println("⚠️ 核心依赖预检查异常，可能在稍后建立通道时失败:", err)
	} else {
		log.Println("✅ 核心引擎准备就绪")
	}

	// 1. 初始化进程与配置管理器
	mgr := NewManager("config.json")
	if err := mgr.Load(); err != nil {
		log.Println("未找到配置文件或解析失败，将创建新的配置: ", err)
	}

	// 1.5 捕获系统退出信号，实现优雅关闭子进程
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("收到退出指令，正在终止所有后台运行的 Gost 进程...")
		mgr.StopAll()
		os.Exit(0)
	}()

	// 2. 准备嵌入的静态前端文件
	var staticFS = fs.FS(webFS)
	htmlContent, err := fs.Sub(staticFS, "web")
	if err != nil {
		log.Fatal("前端文件挂载失败:", err)
	}

	// 3. 配置路由
	// 静态文件路由
	http.Handle("/", http.FileServer(http.FS(htmlContent)))

	// API：获取规则列表和添加新规则
	http.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(mgr.GetRules())
		} else if r.Method == http.MethodPost {
			var rule Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := mgr.AddRule(rule); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
		}
	})

	// API：修改规则
	http.HandleFunc("/api/rules/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mgr.UpdateRule(rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// API：启停规则
	http.HandleFunc("/api/rules/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if req.Enabled {
			if err := mgr.StartRule(req.ID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := mgr.StopRule(req.ID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// API：提取节点执行 TCP 测速
	http.HandleFunc("/api/rules/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		rule, err := mgr.GetRule(req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		target := net.JoinHostPort(rule.RemoteIP, strconv.Itoa(rule.RemotePort))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 3*time.Second)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"latency": -1})
			return
		}
		conn.Close()
		
		json.NewEncoder(w).Encode(map[string]interface{}{"latency": time.Since(start).Milliseconds()})
	})

	// API：删除规则
	http.HandleFunc("/api/rules/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "不允许的请求方法", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := mgr.DeleteRule(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("服务已启动: http://localhost:%s\n", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatal(err)
	}
}
