package hook

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path"

	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/constant"
	"github.com/1Panel-dev/1Panel/agent/global"
)

func Init() {
	settingRepo := repo.NewISettingRepo()
	OneDriveID, err := settingRepo.Get(settingRepo.WithByKey("OneDriveID"))
	if err != nil {
		global.LOG.Errorf("load onedrive info from setting failed, err: %v", err)
	}
	idItem, _ := base64.StdEncoding.DecodeString(OneDriveID.Value)
	global.CONF.System.OneDriveID = string(idItem)
	OneDriveSc, err := settingRepo.Get(settingRepo.WithByKey("OneDriveSc"))
	if err != nil {
		global.LOG.Errorf("load onedrive info from setting failed, err: %v", err)
	}
	scItem, _ := base64.StdEncoding.DecodeString(OneDriveSc.Value)
	global.CONF.System.OneDriveSc = string(scItem)

	if _, err := settingRepo.Get(settingRepo.WithByKey("SystemStatus")); err != nil {
		_ = settingRepo.Create("SystemStatus", "Free")
	}
	if err := settingRepo.Update("SystemStatus", "Free"); err != nil {
		global.LOG.Fatalf("init service before start failed, err: %v", err)
	}

	handleCronjobStatus()
	handleSnapStatus()
	loadLocalDir()
	initDir()
}

func handleSnapStatus() {
	msgFailed := "the task was interrupted due to the restart of the 1panel service"
	_ = global.DB.Model(&model.Snapshot{}).Where("status = ?", "OnSaveData").
		Updates(map[string]interface{}{"status": constant.StatusSuccess}).Error

	_ = global.DB.Model(&model.Snapshot{}).Where("status = ?", constant.StatusWaiting).
		Updates(map[string]interface{}{
			"status":  constant.StatusFailed,
			"message": msgFailed,
		}).Error

	_ = global.DB.Model(&model.Snapshot{}).Where("recover_status = ?", constant.StatusWaiting).
		Updates(map[string]interface{}{
			"recover_status":  constant.StatusFailed,
			"recover_message": msgFailed,
		}).Error

	_ = global.DB.Model(&model.Snapshot{}).Where("rollback_status = ?", constant.StatusWaiting).
		Updates(map[string]interface{}{
			"rollback_status":  constant.StatusFailed,
			"rollback_message": msgFailed,
		}).Error

	snapRepo := repo.NewISnapshotRepo()

	status, _ := snapRepo.GetStatusList()
	for _, item := range status {
		updates := make(map[string]interface{})
		if item.Panel == constant.StatusRunning {
			updates["panel"] = constant.StatusFailed
		}
		if item.PanelInfo == constant.StatusRunning {
			updates["panel_info"] = constant.StatusFailed
		}
		if item.DaemonJson == constant.StatusRunning {
			updates["daemon_json"] = constant.StatusFailed
		}
		if item.AppData == constant.StatusRunning {
			updates["app_data"] = constant.StatusFailed
		}
		if item.PanelData == constant.StatusRunning {
			updates["panel_data"] = constant.StatusFailed
		}
		if item.BackupData == constant.StatusRunning {
			updates["backup_data"] = constant.StatusFailed
		}
		if item.Compress == constant.StatusRunning {
			updates["compress"] = constant.StatusFailed
		}
		if item.Upload == constant.StatusUploading {
			updates["upload"] = constant.StatusFailed
		}
		if len(updates) != 0 {
			_ = snapRepo.UpdateStatus(item.ID, updates)
		}
	}
}

func handleCronjobStatus() {
	_ = global.DB.Model(&model.JobRecords{}).Where("status = ?", constant.StatusWaiting).
		Updates(map[string]interface{}{
			"status":  constant.StatusFailed,
			"message": "the task was interrupted due to the restart of the 1panel service",
		}).Error
}

func loadLocalDir() {
	var backup model.BackupAccount
	_ = global.DB.Where("type = ?", "LOCAL").First(&backup).Error
	if backup.ID == 0 {
		global.LOG.Errorf("no such backup account `%s` in db", "LOCAL")
		return
	}
	varMap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(backup.Vars), &varMap); err != nil {
		global.LOG.Errorf("json unmarshal backup.Vars: %v failed, err: %v", backup.Vars, err)
		return
	}
	if _, ok := varMap["dir"]; !ok {
		global.LOG.Error("load local backup dir failed")
		return
	}
	baseDir, ok := varMap["dir"].(string)
	if ok {
		if _, err := os.Stat(baseDir); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(baseDir, os.ModePerm); err != nil {
				global.LOG.Errorf("mkdir %s failed, err: %v", baseDir, err)
				return
			}
		}
		global.CONF.System.Backup = baseDir
		return
	}
	global.LOG.Errorf("error type dir: %T", varMap["dir"])
}

func initDir() {
	composePath := path.Join(global.CONF.System.BaseDir, "1panel/docker/compose/")
	if _, err := os.Stat(composePath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(composePath, os.ModePerm); err != nil {
			global.LOG.Errorf("mkdir %s failed, err: %v", composePath, err)
			return
		}
	}
}