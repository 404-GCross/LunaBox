package umbra

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
)

const (
	gameSubjectPrefix       = "game_"
	coverSubjectPrefix      = "cover_"
	latestSaveSubjectPrefix = "latest_"
	librarySubject          = "library"
	libraryDirSubjectPrefix = "library_"
	databaseLatestSubject   = "database"
)

type listQuery struct {
	filter umbrsdk.BackupListFilter
	prefix string
}

func addressForSubPath(subPath string) (umbrsdk.BackupAddress, error) {
	clean, err := normalizeSubPath(subPath)
	if err != nil {
		return umbrsdk.BackupAddress{}, err
	}
	parts := strings.Split(clean, "/")

	switch {
	case len(parts) == 3 && parts[0] == "saves" && strings.HasSuffix(parts[2], ".zip"):
		gameID := parts[1]
		version := strings.TrimSuffix(parts[2], ".zip")
		if version == "latest" {
			subject, err := encodeSubject(latestSaveSubjectPrefix, gameID)
			if err != nil {
				return umbrsdk.BackupAddress{}, err
			}
			return validatedAddress(umbrsdk.SyncBackup(subject, "save"))
		}
		subject, err := encodeSubject(gameSubjectPrefix, gameID)
		if err != nil {
			return umbrsdk.BackupAddress{}, err
		}
		return validatedAddress(umbrsdk.GameBackup(subject, version))

	case len(parts) == 2 && parts[0] == "database" && strings.HasSuffix(parts[1], ".zip"):
		version := strings.TrimSuffix(parts[1], ".zip")
		if version == "latest" {
			return validatedAddress(umbrsdk.SyncBackup(databaseLatestSubject, "latest"))
		}
		return validatedAddress(umbrsdk.DBBackup(version))

	case len(parts) == 3 && parts[0] == "sync" && parts[1] == "library" && strings.HasSuffix(parts[2], ".json"):
		return validatedAddress(umbrsdk.SyncBackup(librarySubject, strings.TrimSuffix(parts[2], ".json")))

	case len(parts) == 4 && parts[0] == "sync" && parts[1] == "library" && strings.HasSuffix(parts[3], ".json"):
		subject, err := encodeSubject(libraryDirSubjectPrefix, parts[2])
		if err != nil {
			return umbrsdk.BackupAddress{}, err
		}
		return validatedAddress(umbrsdk.SyncBackup(subject, strings.TrimSuffix(parts[3], ".json")))

	case len(parts) == 3 && parts[0] == "sync" && parts[1] == "covers":
		ext := path.Ext(parts[2])
		gameID := strings.TrimSuffix(parts[2], ext)
		if gameID == "" || ext == "" {
			return umbrsdk.BackupAddress{}, fmt.Errorf("Umbra 不支持的封面路径: %s", subPath)
		}
		subject, err := encodeSubject(coverSubjectPrefix, gameID)
		if err != nil {
			return umbrsdk.BackupAddress{}, err
		}
		return validatedAddress(umbrsdk.AssetBackup(subject, "cover_"+strings.TrimPrefix(ext, ".")))
	default:
		return umbrsdk.BackupAddress{}, fmt.Errorf("Umbra 不支持的云端路径: %s", subPath)
	}
}

func listQueryForSubPath(subPath string) (listQuery, error) {
	clean := strings.Trim(strings.ReplaceAll(subPath, "\\", "/"), "/")
	parts := strings.Split(clean, "/")

	switch {
	case clean == "database":
		return listQuery{filter: umbrsdk.BackupListFilter{Category: umbrsdk.CategoryDB}, prefix: "database/"}, nil
	case len(parts) == 2 && parts[0] == "saves":
		subject, err := encodeSubject(gameSubjectPrefix, parts[1])
		if err != nil {
			return listQuery{}, err
		}
		return listQuery{
			filter: umbrsdk.BackupListFilter{Category: umbrsdk.CategoryGame, Subject: subject},
			prefix: "saves/" + parts[1] + "/",
		}, nil
	case clean == "sync/library":
		return listQuery{filter: umbrsdk.BackupListFilter{Category: umbrsdk.CategorySync}, prefix: "sync/library/"}, nil
	case len(parts) == 3 && parts[0] == "sync" && parts[1] == "library":
		subject, err := encodeSubject(libraryDirSubjectPrefix, parts[2])
		if err != nil {
			return listQuery{}, err
		}
		return listQuery{
			filter: umbrsdk.BackupListFilter{Category: umbrsdk.CategorySync, Subject: subject},
			prefix: clean + "/",
		}, nil
	case clean == "sync/covers":
		return listQuery{filter: umbrsdk.BackupListFilter{Category: umbrsdk.CategoryAsset}, prefix: "sync/covers/"}, nil
	default:
		return listQuery{}, fmt.Errorf("Umbra 不支持列出云端路径: %s", subPath)
	}
}

func subPathForRecord(record umbrsdk.BackupRecord) (string, bool) {
	switch umbrsdk.BackupCategory(record.Category) {
	case umbrsdk.CategoryDB:
		return "database/" + record.Version + ".zip", record.Version != ""
	case umbrsdk.CategoryGame:
		gameID, ok := decodeSubject(gameSubjectPrefix, record.Subject)
		if !ok || record.Version == "" {
			return "", false
		}
		return "saves/" + gameID + "/" + record.Version + ".zip", true
	case umbrsdk.CategoryAsset:
		gameID, ok := decodeSubject(coverSubjectPrefix, record.Subject)
		ext := strings.TrimPrefix(record.Version, "cover_")
		if !ok || ext == "" || ext == record.Version {
			return "", false
		}
		return "sync/covers/" + gameID + "." + ext, true
	case umbrsdk.CategorySync:
		switch {
		case record.Subject == databaseLatestSubject && record.Version == "latest":
			return "database/latest.zip", true
		case record.Subject == librarySubject && record.Version != "":
			return "sync/library/" + record.Version + ".json", true
		case strings.HasPrefix(record.Subject, libraryDirSubjectPrefix) && record.Version != "":
			dir, ok := decodeSubject(libraryDirSubjectPrefix, record.Subject)
			if !ok {
				return "", false
			}
			return "sync/library/" + dir + "/" + record.Version + ".json", true
		case strings.HasPrefix(record.Subject, latestSaveSubjectPrefix) && record.Version == "save":
			gameID, ok := decodeSubject(latestSaveSubjectPrefix, record.Subject)
			if !ok {
				return "", false
			}
			return "saves/" + gameID + "/latest.zip", true
		}
	}
	return "", false
}

func normalizeSubPath(value string) (string, error) {
	value = strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
	if value == "" {
		return "", fmt.Errorf("Umbra 云端路径不能为空")
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("Umbra 云端路径无效: %s", value)
	}
	return clean, nil
}

func encodeSubject(prefix, value string) (string, error) {
	if value == "" || strings.ContainsAny(value, `/\\`) {
		return "", fmt.Errorf("Umbra 路径标识无效: %s", value)
	}
	encoded := prefix + base64.RawURLEncoding.EncodeToString([]byte(value))
	if len(encoded) > 64 {
		return "", fmt.Errorf("Umbra 路径标识过长: %s", value)
	}
	return encoded, nil
}

func decodeSubject(prefix, value string) (string, bool) {
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || len(decoded) == 0 || strings.ContainsAny(string(decoded), `/\\`) {
		return "", false
	}
	return string(decoded), true
}

func validatedAddress(address umbrsdk.BackupAddress) (umbrsdk.BackupAddress, error) {
	if err := umbrsdk.ValidateAddress(address); err != nil {
		return umbrsdk.BackupAddress{}, fmt.Errorf("Umbra 云端路径无法映射: %w", err)
	}
	return address, nil
}
