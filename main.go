package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/openvohive/openvohive/internal/api"
	"github.com/openvohive/openvohive/internal/config"
	"github.com/openvohive/openvohive/internal/db"
	"github.com/openvohive/openvohive/internal/device"
	"github.com/openvohive/openvohive/internal/notify"
	"github.com/openvohive/openvohive/internal/sipgw"

	"github.com/openvohive/openvohive/pkg/logger"
	"github.com/openvohive/openvohive/web"

	"github.com/emiago/sipgo/sip"
)

func main() {
	// 开启 SIP_DEBUG 以排查问题（针对旧系统或备用系统）
	os.Setenv("SIP_DEBUG", "false")

	sipLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	sip.SetDefaultLogger(sipLogger)
	sip.SIPDebug = false
	// 绕过 sipgo 底层硬编码的 UDP MTU 限制（默认 1500），
	// 防止由于包含 APNs/FCM 推送 Token 的超长 Contact URI 导致 UDP 发送直接报错。
	// 大包会自动在 IP 层被切片(IP Fragmentation)。
	sip.UDPMTUSize = 65535
	// Parse flags
	var configPath string
	var backendOnly bool
	flag.StringVar(&configPath, "c", "config/config.yaml", "config file path")
	flag.BoolVar(&backendOnly, "backend-only", false, "run as backend-only (disable embedded web UI)")
	flag.Parse()

	if err := config.InitGlobalManager(configPath); err != nil {
		log.Fatalf("初始化配置管理器失败: %v", err)
	}
	cfg := config.GetConfig()

	logger.Setup(logger.LogConfig{
		Debug:    cfg.Server.Debug,
		Filename: "logs/app.log",
	})
	// 将内置 slog 重定向到已就绪的系统日志框架
	slog.SetDefault(slog.New(logger.NewSlogHandler(logger.ZapLogger())))
	logger.Info("VoHive 模组管理器启动中...")

	dbPath := "data/vohive.db"
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	dbResolvedPath := dbPath
	if absPath, err := filepath.Abs(dbPath); err == nil {
		dbResolvedPath = absPath
	}
	logger.Info("数据库已初始化", "path", dbPath, "resolved_path", dbResolvedPath)

	go func() {
		need, err := db.NeedBackfillSMSContacts()
		if err != nil {
			logger.Error("短信联系人回填检查失败", "err", err)
			return
		}
		if !need {
			return
		}
		logger.Info("开始短信联系人回填")
		if err := db.BackfillSMSPeerAndContacts(1000); err != nil {
			logger.Error("短信联系人回填失败", "err", err)
			return
		}
		logger.Info("短信联系人回填完成")
	}()

	pool := device.NewPool(cfg)

	// 卡策略：注入 db-backed resolver；一次性把旧 yaml 策略种子进 card_policies。
	pool.SetPolicyResolver(db.CardPolicyResolver{})

	legacy, err := config.ReadLegacyDevicePoliciesFromYAML(configPath, func(deviceID string) string {
		return db.CurrentICCIDForDevice(deviceID)
	})
	if err == nil {
		var count int64
		db.DB.Model(&db.CardPolicy{}).Count(&count)
		if count == 0 { // 仅当 policy 表为空时迁移
			n, _ := config.SeedLegacyDevicePolicies(legacy, func(iccid string, p config.LegacyDevicePolicy) error {
				policy := db.DefaultCardPolicy(iccid)
				policy.NetworkEnabled = p.NetworkEnabled
				policy.IPVersion = p.IPVersion
				policy.APN = p.APN
				return db.UpsertCardPolicy(policy)
			})
			logger.Info("卡策略种子迁移完成", "count", n)
		}
	} else {
		logger.Warn("读取旧 yaml 策略失败，跳过种子迁移", "err", err)
	}

	notifyMgr, err := notify.NewManager(cfg, pool)
	if err != nil {
		logger.Warn("通知管理器初始化异常", "err", err)
	} else {
		pool.SetNotifier(notifyMgr)
	}

	var sipRegistrar *sipgw.Registrar
	if cfg.VoiceGateway.SIP.Listen != "" {
		sipgwCfg := sipgw.Config{
			Enabled: true,
			SIP: sipgw.SIPConfig{
				Listen:     cfg.VoiceGateway.SIP.Listen,
				Transport:  cfg.VoiceGateway.SIP.Transport,
				Realm:      cfg.VoiceGateway.SIP.Realm,
				ExternalIP: cfg.VoiceGateway.SIP.ExternalIP,
			},
			Media: sipgw.MediaConfig{
				RTPPortMin: cfg.VoiceGateway.Media.RTPPortMin,
				RTPPortMax: cfg.VoiceGateway.Media.RTPPortMax,
				Codecs:     cfg.VoiceGateway.Media.Codecs,
			},
			LinphonePush: sipgw.LinphonePushConfig{
				LinphoneUser:     cfg.VoiceGateway.LinphonePush.LinphoneUser,
				LinphonePassword: cfg.VoiceGateway.LinphonePush.LinphonePassword,
			},
		}
		for _, u := range cfg.VoiceGateway.Users {
			sipgwCfg.Users = append(sipgwCfg.Users, sipgw.UserConfig{
				Username:    u.Username,
				Password:    u.Password,
				DisplayName: u.DisplayName,
				DeviceID:    u.DeviceID,
			})
		}
		if sipgwCfg.SIP.Transport == "" {
			sipgwCfg.SIP.Transport = "udp"
		}
		if sipgwCfg.SIP.Realm == "" {
			sipgwCfg.SIP.Realm = "vohive.local"
		}
		if sipgwCfg.Media.RTPPortMin == 0 {
			sipgwCfg.Media.RTPPortMin = 10000
		}
		if sipgwCfg.Media.RTPPortMax == 0 {
			sipgwCfg.Media.RTPPortMax = 20000
		}

		sipRegistrar, err = sipgw.NewRegistrar(sipgwCfg)
		if err != nil {
			logger.Error("Registrar 初始化失败", "err", err)
		} else {
			sipRegistrar.SetOnInvite(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
				if w := pool.GetWorker(deviceID); w != nil && w.CSCallMgr != nil {
					w.CSCallMgr.HandleOutboundInvite(deviceID, req, tx)
				} else {
					logger.Error("Service Unavailable - No Call Manager")
					tx.Respond(sip.NewResponseFromRequest(req, 503, "Service Unavailable - No Call Manager", nil))
				}
			})
			sipRegistrar.SetOnCancel(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
				callID := req.CallID().Value()
				if w := pool.GetWorker(deviceID); w != nil && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
					w.CSCallMgr.HandleClientCancel(callID)
					tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
					return
				}
				tx.Respond(sip.NewResponseFromRequest(req, 481, "Call/Transaction Does Not Exist", nil))
			})
			sipRegistrar.SetOnBye(func(deviceID string, req *sip.Request, tx sip.ServerTransaction) {
				callID := req.CallID().Value()
				if w := pool.GetWorker(deviceID); w != nil && w.CSCallMgr != nil && w.CSCallMgr.HasCall(callID) {
					w.CSCallMgr.HandleClientBye(callID)
					return
				}
			})
			pool.SetSIPRegistrar(sipRegistrar)
			if err := sipRegistrar.Start(context.Background()); err != nil {
				logger.Error("Registrar 启动失败", "err", err)
			} else {
				logger.Info("软电话 Registrar 已启动", "listen", sipgwCfg.SIP.Listen, "users", len(sipgwCfg.Users))
			}
		}
	}

	_ = pool.StartAll()

	var staticFS http.FileSystem
	if backendOnly {
		logger.Info("启用纯后端模式（未挂载前端静态资源）")
	} else {
		distFS, err := web.GetFS()
		if err != nil {
			log.Fatalf("无法加载嵌入的 Web 文件: %v", err)
		}
		staticFS = http.FS(distFS)
	}

	apiServer := api.New(cfg, pool, staticFS, notifyMgr, configPath)

	apiErrCh := make(chan error, 1)
	go func() {
		if err := apiServer.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			apiErrCh <- err
		}
	}()

	logger.Info("所有服务已启动")

	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	var sig os.Signal
	select {
	case sig = <-quit:
		logger.Info("收到关闭信号", "signal", sig.String())
	case err := <-apiErrCh:
		logger.Error("API 服务器失败", "err", err)
	}
	logger.Info("正在优雅关闭所有服务...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("关闭 API 服务器时出错", "err", err)
		}

		if notifyMgr != nil {
			notifyMgr.Close()
		}

		if sipRegistrar != nil {
			if err := sipRegistrar.Stop(); err != nil {
				logger.Error("关闭 Registrar 时出错", "err", err)
			}
		}

		if err := pool.Shutdown(); err != nil {
			logger.Error("关闭工作器池时出错", "err", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-quit:
	case <-time.After(12 * time.Second):
		logger.Warn("关闭超时，强制退出")
	}

	logger.Info("再见!")
}
