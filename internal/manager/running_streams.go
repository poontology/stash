package manager

import (
	"net/http"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

func KillRunningStreams(scene *models.Scene, fileNamingAlgo models.HashAlgorithm) {
	instance.ReadLockManager.Cancel(scene.Path)

	sceneHash := scene.GetHash(fileNamingAlgo)

	if sceneHash == "" {
		return
	}

	transcodePath := GetInstance().Paths.Scene.GetTranscodePath(sceneHash)
	instance.ReadLockManager.Cancel(transcodePath)
}

type SceneServer struct {
	TXNManager models.TransactionManager
}

func (s *SceneServer) StreamSceneDirect(scene *models.Scene, w http.ResponseWriter, r *http.Request) {
	fileNamingAlgo := config.GetInstance().GetVideoFileNamingAlgorithm()

	filepath := GetInstance().Paths.Scene.GetStreamPath(scene.Path, scene.GetHash(fileNamingAlgo))
	lockCtx := GetInstance().ReadLockManager.ReadLock(r.Context(), filepath)
	defer lockCtx.Cancel()
	http.ServeFile(w, r, filepath)
}

func (s *SceneServer) ServeScreenshot(scene *models.Scene, w http.ResponseWriter, r *http.Request) {
	checksum := scene.GetHash(config.GetInstance().GetVideoFileNamingAlgorithm())
	filepath := GetInstance().Paths.Scene.GetScreenshotPath(checksum)

	thumbnailQueryParam := r.URL.Query().Get("thumbnail")
	if thumbnailQueryParam != "" {
		filepath = GetInstance().Paths.Scene.GetThumbnailScreenshotPath(checksum)
	}

	// fall back to the scene image blob if the file isn't present
	screenshotExists, _ := fsutil.FileExists(filepath)
	if screenshotExists {
		http.ServeFile(w, r, filepath)
	} else {
		var cover []byte
		err := s.TXNManager.WithReadTxn(r.Context(), func(repo models.ReaderRepository) error {
			cover, _ = repo.Scene().GetCover(scene.ID)
			return nil
		})
		if err != nil {
			logger.Warnf("read transaction failed while serving screenshot: %v", err)
		}

		if err = utils.ServeImage(cover, w, r); err != nil {
			logger.Warnf("unable to serve screenshot image: %v", err)
		}
	}
}
