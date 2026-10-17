package test

import (
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
)

func addGameViaMetadata(gameService *service.GameService, game models.Game) error {
	source := game.SourceType
	if source == "" {
		source = enums.Local
	}

	return gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
		Source: source,
		Game:   game,
	})
}
