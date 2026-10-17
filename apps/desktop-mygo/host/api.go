package host

import (
	"database/sql"

	"lunabox/backend/appconf"
	"lunabox/backend/bootstrap"
	"lunabox/desktop-mygo/api"
)

// Backend 是界面使用的后端：bootstrap 构造的服务、数据库与配置，加上把服务事件交给界面的 Events。
// Config 是应用配置（遮住了 Services.Config 这个 ConfigService，后者用 Services.Config 访问）。
type Backend struct {
	*bootstrap.Services
	DB      *sql.DB
	Config  *appconf.AppConfig
	DataDir string
	Events  *Events
}

// API 返回界面使用的接口，背后是同一组真实服务。
func (b *Backend) API() *api.Services {
	s := b.Services
	return &api.Services{
		Config:  b.Config,
		DataDir: b.DataDir,
		Events:  b.Events,

		AppConfig:   s.Config,
		Home:        s.Home,
		Game:        s.Game,
		Tag:         s.Tag,
		Stats:       s.Stats,
		Session:     s.Session,
		Category:    s.Category,
		Backup:      s.Backup,
		Start:       s.Start,
		Integration: s.Integration,
		Progress:    s.GameProgress,
		Review:      s.GameReview,
		Bangumi:     s.Bangumi,
		Hikarinagi:  s.Hikarinagi,
		Kungal:      s.Kungal,
		Download:    s.Download,
		Version:     s.Version,
		Update:      s.Update,
		Portable:    s.PortableSetup,
	}
}
