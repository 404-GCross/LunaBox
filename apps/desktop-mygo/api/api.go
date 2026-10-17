// Package api 是界面调用的后端接口，对应原版前端调用的 Wails 绑定（frontend/bindings/lunabox/backend/service）。
// 界面只依赖这里的接口，由 host 用 packages/backend 的真实服务实现（host.Backend.API）。
// 方法与 packages/backend/service 同名同签名；界面用到新的服务方法时在这里补上。
package api

import (
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	launcherpkg "lunabox/backend/service/launcher"
	"lunabox/backend/utils/processutils"
)

// Services 是界面用到的全部后端能力，各页面通过 env.Env.Svc 使用。
type Services struct {
	Config  *appconf.AppConfig // 与服务共用的当前配置
	DataDir string             // 数据目录（封面、背景等本地文件相对它）
	Events  Events             // 服务发出的事件

	AppConfig   ConfigService
	Game        GameService
	Tag         TagService
	Stats       StatsService
	Session     SessionService
	Category    CategoryService
	Backup      BackupService
	Start       StartService
	Integration IntegrationService
	Progress    GameProgressService
	Review      GameReviewService
	Bangumi     AccountService[vo.BangumiAuthStatus, vo.BangumiProfile]
	Hikarinagi  AccountService[vo.HikarinagiAuthStatus, vo.HikarinagiProfile]
	Kungal      AccountService[vo.KungalAuthStatus, vo.KungalProfile]
	Download    DownloadService
	Version     VersionService
	Update      UpdateService
	Portable    PortableSetupService
}

// Events 是服务事件的订阅（原版的 Events.On）：fn 总在窗口的主线程上调用，data 是事件的第一个参数。
type Events interface {
	On(name string, fn func(data any))
}

// ConfigService 对应 service.ConfigService。
type ConfigService interface {
	GetAppConfig() (appconf.AppConfig, error)
	UpdateAppConfig(newConfig appconf.AppConfig) error
	SafeQuit()
	SelectDirectory(title string) (string, error)
	SelectAndCropBackgroundImage() (string, error)
	SaveCroppedBackgroundImage(srcPath string, x, y, width, height int) (string, error)
	PreviewGameLibraryPathChange(newPath string) (service.GameLibraryPathChangePreview, error)
	ApplyGameLibraryPathChange(newPath string, syncPaths bool) (service.GameLibraryPathChangeResult, error)
	ExportLogsZip() (string, error)
	OpenDataDirectory() (string, error)
}

// GameService 对应 service.GameService。
type GameService interface {
	GetGameByID(id string) (models.Game, error)
	UpdateGame(game models.Game) error
	DeleteGame(id string) error
	BatchUpdateStatus(ids []string, status string) error
	UpdateGameProcessName(gameID string, processName string) error
	GetRunningProcesses() ([]processutils.ProcessInfo, error)

	SelectGameExecutable(currentPath string) (string, error)
	SelectWineRunnerExecutable(currentPath string) (string, error)
	SelectGameDirectory(currentPath string) (string, error)
	SelectSaveDirectory(rootPath string) (string, error)
	SelectSaveFile(rootPath string) (string, error)
	SelectCoverImage(gameID string) (string, error)
	SaveCoverImageDataURL(gameID string, dataURL string) (string, error)
	DownloadCoverImage(gameID string, coverURL string) (string, error)
	ExportCoverImage(gameID string) (string, error)
	ExportLaunchShortcut(gameID string) (string, error)
	OpenLocalPath(path string) error
	FindGameGuideDocuments(gameID string) ([]vo.GameGuideDocument, error)
	OpenGameGuideDocument(gameID string, relativePath string) error

	FetchMetadataByName(name string) ([]vo.GameMetadataFromWebVO, error)
	UpdateGameFromRemoteBySource(gameID string, source enums.SourceType) error
	UpdateGameFromRemoteWithFields(gameID string, fields []enums.MetadataUpdateField) error
	RefreshGamesMetadataWithFields(gameIDs []string, fields []enums.MetadataUpdateField) (vo.MetadataRefreshResult, error)
	RefreshAllGamesMetadataWithFields(fields []enums.MetadataUpdateField) (vo.MetadataRefreshResult, error)
	SetDefaultMetadataSource(gameID string, source enums.SourceType) error
	UpsertGameMetadataSource(gameID string, source enums.SourceType, sourceID string) error
	DeleteGameMetadataSource(gameID string, source enums.SourceType) error
	PreviewLegacyGameMetadataSourceIDs() (service.GameIDEnrichmentPreview, error)
	EnrichLegacyGameMetadataSourceIDs() (service.GameIDEnrichmentResult, error)
	StartRemoteCoverImageDownloadTask() (string, error)
}

// TagService 对应 service.TagService。
type TagService interface {
	GetTagsByGame(gameID string) ([]models.GameTag, error)
	AddUserTag(gameID string, tagName string) error
	DeleteTag(tagID string) error
}

// StatsService 对应 service.StatsService。
type StatsService interface {
	GetGameStats(req vo.GameStatsRequest) (vo.GameDetailStats, error)
}

// SessionService 对应 service.SessionService。
type SessionService interface {
	GetPlaySessions(gameID string) ([]models.PlaySession, error)
	AddPlaySession(gameID string, startTime time.Time, durationSeconds int) (models.PlaySession, error)
	DeletePlaySession(sessionID string) error
}

// CategoryService 对应 service.CategoryService。
type CategoryService interface {
	GetCategories() ([]vo.CategoryVO, error)
	GetCategoryByID(id string) (vo.CategoryVO, error)
	GetCategoriesByGame(gameID string) ([]vo.CategoryVO, error)
	GetCategoryGames(req vo.CategoryGameListRequest) (vo.GameListResponse, error)
	SearchCategoryGameCandidates(req vo.CategoryGameCandidateRequest) (vo.GameListResponse, error)
	AddCategory(name string, emoji string) error
	UpdateCategory(id, name, emoji string) error
	DeleteCategory(id string) error
	DeleteCategories(ids []string) error
	AddGameToCategory(gameID, categoryID string) error
	RemoveGameFromCategory(gameID, categoryID string) error
	RemoveGamesFromCategory(gameIDs []string, categoryID string) error
}

// BackupService 对应 service.BackupService：游戏存档备份、数据库与完整数据备份、云备份账号。
type BackupService interface {
	GetGameBackups(gameID string) ([]models.GameBackup, error)
	CreateBackup(gameID string) (*models.GameBackup, error)
	RestoreBackup(backupPath string) error
	DeleteBackup(backupPath string) error
	OpenBackupFolder(gameID string) error
	GetCloudBackupStatus() vo.CloudBackupStatus
	GetCloudGameBackups(gameID string) ([]vo.CloudBackupItem, error)
	UploadGameBackupToCloud(gameID string, backupPath string) error
	RestoreFromCloud(cloudKey string, gameID string) error

	GetDBBackups() (*vo.DBBackupStatus, error)
	GetCloudDBBackups() ([]vo.CloudBackupItem, error)
	CreateAndUploadDBBackup() (*vo.DBBackupInfo, error)
	CreateAndUploadDBBackupForQuit() (*vo.DBBackupInfo, error)
	UploadDBBackupToCloud(backupPath string) error
	DeleteDBBackup(backupPath string) error
	ScheduleDBRestore(backupPath string) error
	ScheduleDBRestoreFromCloud(cloudKey string) error

	SelectBackupSavePath() (string, error)
	SelectBackupRestorePath() (string, error)
	CreateFullDataBackup(savePath string) error
	ScheduleFullDataRestore(backupPath string) error

	SetupCloudBackup(password string) (string, error)
	TestS3Connection(config appconf.AppConfig) error
	TestOneDriveConnection(config appconf.AppConfig) error
	TestWebDAVConnection(config appconf.AppConfig) error
	TestUmbraConnection(config appconf.AppConfig) error
	StartOneDriveAuth(clientID string) (string, error)
	StartUmbraAuth(config appconf.AppConfig) error
	CancelUmbraAuth() bool
	GetUmbraUserProfile(config appconf.AppConfig) (*vo.UmbraUserProfile, error)
	LogoutUmbra(config appconf.AppConfig) error
}

// StartService 对应 service.StartService。
type StartService interface {
	StartGameWithOptions(gameID string, options launcherpkg.LaunchOptions) (bool, error)
}

// IntegrationService 对应 service.IntegrationService（Steam 集成）。
type IntegrationService interface {
	GetGameSteamStatus(gameID string) (service.SteamLaunchStatus, error)
	SetGameSteamLaunchOptions(gameID string, launchOptions string) (service.SteamLaunchStatus, error)
	ImportGameToSteam(gameID string) (service.SteamImportResult, error)
	RestartSteamClient() error
}

// GameProgressService 对应 service.GameProgressService。
type GameProgressService interface {
	GetGameProgress(gameID string) (*models.GameProgress, error)
	ListGameProgresses(gameID string) ([]models.GameProgress, error)
	UpsertGameProgress(gp models.GameProgress) (*models.GameProgress, error)
}

// GameReviewService 对应 service.GameReviewService。
type GameReviewService interface {
	GetGameReview(gameID string) (*models.GameReview, error)
	SaveGameReview(review models.GameReview) (*models.GameReview, error)
	SyncGameReview(gameID string, providers []enums.SourceType) (vo.GameReviewSyncResult, error)
}

// AccountService 是 Bangumi、Hikarinagi、Kungal 三个账号来源共有的授权与同步方法，Status、Profile 是各自的 vo 类型。
type AccountService[Status, Profile any] interface {
	GetAuthStatus() (Status, error)
	GetProfile() (Profile, error)
	StartAuth() (Status, error)
	Disconnect() (Status, error)
	SyncAllGameStatuses() (vo.RemoteStatusSyncProgress, error)
}

// DownloadService 对应 service.DownloadService。
type DownloadService interface {
	GetDownloadTasks() []service.DownloadTask
	CheckDownloadImportStates(requests []vo.DownloadImportStateRequest) ([]vo.DownloadImportState, error)
	PauseDownload(taskID string) error
	ResumeDownload(taskID string) error
	RetryDownload(taskID string) error
	CancelDownload(taskID string) error
	DeleteDownloadTask(taskID string) error
	OpenDownloadTaskLocation(taskID string) error
	ImportDownloadTaskAsGame(taskID string) error
}

// VersionService 对应 service.VersionService。
type VersionService interface {
	GetVersionInfo() map[string]string
	GetGOOS() string
	SupportsBackgroundProcessMute() bool
}

// UpdateService 对应 service.UpdateService。
type UpdateService interface {
	CheckForUpdates() (*service.UpdateCheckResult, error)
	SkipVersion(ver string) error
}

// PortableSetupService 对应 service.PortableSetupService。
type PortableSetupService interface {
	GetStatus() (service.PortableSetupStatus, error)
	RegisterProtocol() (service.PortableSetupStatus, error)
	UnregisterProtocol() (service.PortableSetupStatus, error)
	RegisterCLIPath() (service.PortableSetupStatus, error)
	UnregisterCLIPath() (service.PortableSetupStatus, error)
}
