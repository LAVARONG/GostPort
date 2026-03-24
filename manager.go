package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

// Rule 定义了单条转发规则
type Rule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Enabled    bool   `json:"enabled"`
	Error      string `json:"error"`   // 报错信息，供前端展示此代理是否崩溃
}

// Manager 集中管理配置与 Gost 进程
type Manager struct {
	ConfigFile string
	rules      map[string]*Rule
	processes  map[string]*exec.Cmd
	lock       sync.Mutex
}

// NewManager 初始化管理器
func NewManager(configFile string) *Manager {
	return &Manager{
		ConfigFile: configFile,
		rules:      make(map[string]*Rule),
		processes:  make(map[string]*exec.Cmd),
	}
}

// Load 从磁盘加载配置并拉起之前处于启用状态的映射
func (m *Manager) Load() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	data, err := os.ReadFile(m.ConfigFile)
	if err != nil {
		return err
	}
	
	var rulesList []Rule
	if err := json.Unmarshal(data, &rulesList); err != nil {
		return err
	}

	for _, r := range rulesList {
		rule := r
		m.rules[rule.ID] = &rule
		// 如果记录显示该规则处于启用状态，则尝试启动它
		if rule.Enabled {
			go func(rl *Rule) {
				// 通过协程错峰启动，减少启动瞬间拥堵
				m.StartRule(rl.ID)
			}(&rule)
		}
	}
	return nil
}

// Save 将当前规则对象持久化保存到磁盘下的 JSON 文件
func (m *Manager) Save() error {
	var rulesList []Rule
	for _, r := range m.rules {
		rulesList = append(rulesList, *r)
	}
	data, err := json.MarshalIndent(rulesList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.ConfigFile, data, 0644)
}

// GetRules 返回供前端展示的规则列表
func (m *Manager) GetRules() []Rule {
	m.lock.Lock()
	defer m.lock.Unlock()
	var list []Rule
	for _, r := range m.rules {
		list = append(list, *r)
	}
	return list
}

// 生成一个简单的随机唯一标识符
func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// AddRule 添加一条新的未启用的规则
func (m *Manager) AddRule(r Rule) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	
	r.ID = generateID()
	r.Enabled = false
	r.Error = ""
	m.rules[r.ID] = &r
	return m.Save()
}

// UpdateRule 更新一条规则
func (m *Manager) UpdateRule(r Rule) error {
	m.lock.Lock()
	old, ok := m.rules[r.ID]
	if !ok {
		m.lock.Unlock()
		return errors.New("未能找到指定的规则记录")
	}

	needsRestart := (old.LocalIP != r.LocalIP) || (old.LocalPort != r.LocalPort) || (old.RemoteIP != r.RemoteIP) || (old.RemotePort != r.RemotePort)

	if !needsRestart {
		// 如果只改了备注等基础信息，无需重启进程，仅更新数据
		old.Name = r.Name
		m.Save()
		m.lock.Unlock()
		return nil
	}

	wasEnabled := old.Enabled
	if wasEnabled {
		if p, ok := m.processes[r.ID]; ok && p.Process != nil {
			p.Process.Kill()
			delete(m.processes, r.ID)
		}
	}

	r.Enabled = false
	r.Error = ""
	m.rules[r.ID] = &r
	m.Save()
	m.lock.Unlock()

	if wasEnabled {
		return m.StartRule(r.ID)
	}
	return nil
}

// DeleteRule 删除某条规则（如果正在运行则先杀掉进程）
func (m *Manager) DeleteRule(id string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	
	if _, ok := m.rules[id]; !ok {
		return errors.New("未能找到指定的规则记录")
	}
	// 尝试终止对应的 Gost 进程
	if p, ok := m.processes[id]; ok && p.Process != nil {
		p.Process.Kill()
		delete(m.processes, id)
	}
	delete(m.rules, id)
	return m.Save()
}

// StartRule 启动某一条转发规则（调用外部 gost 程序）
func (m *Manager) StartRule(id string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	
	rule, ok := m.rules[id]
	if !ok {
		return errors.New("寻找不到对应的映射")
	}
	
	// 先行清洗残留的进程
	if p, ok := m.processes[id]; ok && p.Process != nil {
		p.Process.Kill()
		delete(m.processes, id)
	}

	rule.Error = ""
	
	// 尝试解析本地 gost，若不存在且不在环境变量中，则全自动从 GitHub 下载
	gostBin, errEnsure := EnsureGost()
	if errEnsure != nil {
		rule.Enabled = false
		rule.Error = "依赖获取失败: " + errEnsure.Error()
		m.Save()
		return errEnsure
	}

	localIP := rule.LocalIP
	if localIP == "" {
		localIP = "0.0.0.0"
	}
	// 组装 Gost 参数： -L=tcp://本地IP:本地端口/目标IP:目标端口
	bind := fmt.Sprintf("tcp://%s:%d/%s:%d", localIP, rule.LocalPort, rule.RemoteIP, rule.RemotePort)
	cmd := exec.Command(gostBin, "-L="+bind)
	
	log.Printf("[开启] 通道 %s -> 监听 %s 转发至 %s:%d\n", rule.Name, bind, rule.RemoteIP, rule.RemotePort)

	// 使用 Start() 而不是 Run()，以便异步挂在后台运行
	if err := cmd.Start(); err != nil {
		rule.Enabled = false
		rule.Error = "启动失败: " + err.Error()
		// 如果缺少 gost.exe 会在这里抛出错误，我们不 panic 返回异常
		m.Save() 
		return err
	}

	m.processes[id] = cmd
	rule.Enabled = true
	m.Save()

	// 开一个协程监视此进程的状态。若由于绑定地址冲突等原因闪退，要能捕获
	go func(id string, c *exec.Cmd) {
		err := c.Wait()
		
		m.lock.Lock()
		defer m.lock.Unlock()
		
		// 只有当前活动的 cmd 依然是这颗进程时，才认为是进程生命周期由于外部原因终结
		if currentCmd, ok := m.processes[id]; ok && currentCmd == c {
			if r, ok := m.rules[id]; ok {
				if r.Enabled {
					r.Enabled = false
					if err != nil {
						r.Error = "进程运行时崩溃"
						log.Printf("[异常拦截] 通道 %s (ID: %s) 意外崩溃已挂断: %v\n", r.Name, r.ID, err)
					} else {
						r.Error = "进程异常结束"
					}
					delete(m.processes, id)
					m.Save()
				}
			}
		}
	}(id, cmd)

	return nil
}

// StopRule 停止转发
func (m *Manager) StopRule(id string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	
	rule, ok := m.rules[id]
	if !ok {
		return errors.New("寻找不到对应的映射")
	}

	if p, ok := m.processes[id]; ok && p.Process != nil {
		p.Process.Kill()
		delete(m.processes, id)
	}
	
	log.Printf("[关闭] 通道 %s (ID: %s) 已手动切断\n", rule.Name, rule.ID)
	
	rule.Enabled = false
	rule.Error = ""
	return m.Save()
}

// StopAll 优雅终止所有底层 Gost 子进程
func (m *Manager) StopAll() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for id, p := range m.processes {
		if p != nil && p.Process != nil {
			p.Process.Kill()
		}
		delete(m.processes, id)
	}
}
