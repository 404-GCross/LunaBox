package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/service/integrator"
	"strings"
)

type SteamLaunchStatus struct {
	State          string `json:"state"`
	Ready          bool   `json:"ready"`
	SteamInstalled bool   `json:"steam_installed"`
	SteamRunning   bool   `json:"steam_running"`
	LaunchID       string `json:"launch_id"`
	LaunchKind     string `json:"launch_kind"`
	UserID         string `json:"user_id"`
}

type SteamImportResult struct {
	Status     SteamLaunchStatus `json:"status"`
	Imported   bool              `json:"imported"`
	BackupPath string            `json:"backup_path"`
}

type IntegrationService struct {
	ctx         context.Context
	db          *sql.DB
	gameService *GameService
}

func NewIntegrationService() *IntegrationService {
	return &IntegrationService{}
}

//wails:ignore
func (s *IntegrationService) Init(ctx context.Context, db *sql.DB, _ *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
}

//wails:ignore
func (s *IntegrationService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

func (s *IntegrationService) GetGameSteamStatus(gameID string) (SteamLaunchStatus, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamLaunchStatus{}, err
	}
	result, err := integrator.ResolveSteamTarget(s.ctx, game)
	if err != nil {
		return SteamLaunchStatus{}, err
	}
	status := steamLaunchStatusFromIntegrator(result.Status)
	if status.Ready {
		if err := s.persistSteamIdentity(game.ID, status); err != nil {
			return SteamLaunchStatus{}, err
		}
	}
	return status, nil
}

func (s *IntegrationService) ImportGameToSteam(gameID string) (SteamImportResult, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamImportResult{}, err
	}
	result, err := integrator.ImportSteamShortcut(s.ctx, game)
	if err != nil {
		return SteamImportResult{}, err
	}
	status := steamLaunchStatusFromIntegrator(result.Status)
	if status.Ready {
		if err := s.persistSteamIdentity(game.ID, status); err != nil {
			return SteamImportResult{}, err
		}
	}
	return SteamImportResult{
		Status:     status,
		Imported:   result.Imported,
		BackupPath: result.BackupPath,
	}, nil
}

func (s *IntegrationService) getGame(gameID string) (models.Game, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return models.Game{}, fmt.Errorf("game ID is required")
	}
	if s.gameService == nil {
		return models.Game{}, fmt.Errorf("game service is not initialized")
	}
	return s.gameService.GetGameByID(gameID)
}

func (s *IntegrationService) persistSteamIdentity(gameID string, status SteamLaunchStatus) error {
	if s.db == nil {
		return fmt.Errorf("Steam integration service is not initialized")
	}
	_, err := s.db.ExecContext(s.ctx, `
		UPDATE games
		SET steam_launch_id = ?,
		    steam_launch_kind = ?,
		    steam_user_id = ?
		WHERE id = ?
	`, status.LaunchID, status.LaunchKind, status.UserID, gameID)
	if err != nil {
		return fmt.Errorf("save Steam launch identity: %w", err)
	}
	return nil
}

func steamLaunchStatusFromIntegrator(status integrator.SteamLaunchStatus) SteamLaunchStatus {
	return SteamLaunchStatus{
		State:          status.State,
		Ready:          status.Ready,
		SteamInstalled: status.SteamInstalled,
		SteamRunning:   status.SteamRunning,
		LaunchID:       status.LaunchID,
		LaunchKind:     status.LaunchKind,
		UserID:         status.UserID,
	}
}
