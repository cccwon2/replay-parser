package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/repparser"
)

func main() {
	r := gin.Default()

	// 모든 도메인에서 CORS 허용
	r.Use(cors.Default())

	r.POST("/analyze", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			log.Printf("File upload error: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file is received"})
			return
		}
		log.Printf("Received file: %s\n", file.Filename)

		// 업로드된 파일 열기
		f, err := file.Open()
		if err != nil {
			log.Printf("Error opening file: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error opening file"})
			return
		}
		defer f.Close()

		// 임시 파일 생성
		tempFile, err := os.CreateTemp("", "replay-*.rep")
		if err != nil {
			log.Printf("Error creating temp file: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating temp file"})
			return
		}
		defer os.Remove(tempFile.Name()) // 임시 파일 삭제
		defer tempFile.Close()

		// multipart.File의 내용을 임시 파일에 저장
		_, err = io.Copy(tempFile, f)
		if err != nil {
			log.Printf("Error copying to temp file: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error writing to temp file"})
			return
		}

		// 리플레이 파일 파싱
		replay, err := repparser.ParseFile(tempFile.Name())
		if err != nil {
			log.Printf("Error parsing replay: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing replay"})
			return
		}

		// 빌드 오더 추출
		var buildOrders []gin.H

		// 명령에서 유닛 생성 및 건물 건설 이벤트를 추출
		for _, cmd := range replay.Commands.Cmds {
			switch action := cmd.(type) {
			case *repcmd.BuildCmd:
				// 건물 건설 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Build %s", action.Unit.String()),
					"type": "Build",
				}
				buildOrders = append(buildOrders, buildOrder)

			case *repcmd.TrainCmd:
				// 유닛 생성 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Train %s", action.Unit.String()),
					"type": "Train",
				}
				buildOrders = append(buildOrders, buildOrder)
			case *repcmd.BuildingMorphCmd:
				// 건물 변형 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Morph Building to %s", action.Unit.String()),
					"type": "BuildingMorph",
				}
				buildOrders = append(buildOrders, buildOrder)
			case *repcmd.CancelTrainCmd:
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Cancel Train Unit (Tag: %x)", action.UnitTag),
					"type": "CancelTrain",
				}
				buildOrders = append(buildOrders, buildOrder)

			case *repcmd.UpgradeCmd:
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Start Upgrade: %s", action.Upgrade.String()),
					"type": "Upgrade",
				}
				buildOrders = append(buildOrders, buildOrder)

			case *repcmd.TechCmd:
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Research Tech: %s", action.Tech.String()),
					"type": "Tech",
				}
				buildOrders = append(buildOrders, buildOrder)
			}
		}

		// 분석 결과 반환
		result := gin.H{
			"gameVersion": replay.Header.Version,
			"mapName":     replay.Header.Map,
			"players":      replay.Header.Players,
			"buildOrders": buildOrders, // 실제 빌드 오더 정보 추가
		}

		c.JSON(http.StatusOK, result)
	})

	r.Run(":9090") // Go 서버는 9090 포트에서 실행
}
