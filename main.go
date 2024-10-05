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
	// Gin을 release 모드로 설정 (프로덕션 환경에 적합)
	gin.SetMode(gin.ReleaseMode)

	// Gin 엔진 생성 및 CORS 설정
	r := gin.Default()
	r.Use(cors.Default())

	// 신뢰할 프록시 IP 설정 (모든 프록시 신뢰 비활성화)
	// 여기서는 프록시 설정을 빈 리스트로 설정 (추후 필요시 프록시 IP 추가 가능)
	r.SetTrustedProxies(nil)

	r.POST("/analyze", func(c *gin.Context) {
		// 파일을 form-data에서 가져옴
		file, err := c.FormFile("file")
		if err != nil {
			log.Printf("File upload error: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file is received"})
			return
		}
		log.Printf("Received file: %s\n", file.Filename)

		// 파일 열기
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

		// 게임 이벤트 추출
		var gameEvents []gin.H

		// 명령에서 모든 종류의 게임 내 이벤트를 추출
		for _, cmd := range replay.Commands.Cmds {
			switch action := cmd.(type) {
			case *repcmd.BuildCmd:
				// 건물 건설 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Build %s", action.Unit.String()),
					"type":   "Build",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.TrainCmd:
				// 유닛 생성 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Train %s", action.Unit.String()),
					"type":   "Train",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.BuildingMorphCmd:
				// 건물 변형 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Morph Building to %s", action.Unit.String()),
					"type":   "BuildingMorph",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.CancelTrainCmd:
				// 유닛 훈련 취소 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Cancel Train Unit (Tag: %x)", action.UnitTag),
					"type":   "CancelTrain",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.UpgradeCmd:
				// 업그레이드 시작 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Start Upgrade: %s", action.Upgrade.String()),
					"type":   "Upgrade",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.TechCmd:
				// 기술 연구 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Research Tech: %s", action.Tech.String()),
					"type":   "Tech",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.TargetedOrderCmd:
				// 유닛 명령 이벤트
				orderType := repcmd.OrderByID(byte(action.Order.ID))
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Order: %s", orderType.String()),
					"type":   "Order",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.HotkeyCmd:
				// 핫키 이벤트
				hotkeyType := repcmd.HotkeyTypeByID(action.HotkeyType.ID)
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Hotkey: %s (Group: %d)", hotkeyType.String(), action.Group),
					"type":   "Hotkey",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.SelectCmd:
				// 유닛 선택 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Select %d units", len(action.UnitTags)),
					"type":   "Select",
				}
				gameEvents = append(gameEvents, buildOrder)

			case *repcmd.LandCmd:
				// 건물 착륙 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Land %s at (%d, %d)", action.Unit.String(), action.Pos.X, action.Pos.Y),
					"type":   "Land",
				}
				gameEvents = append(gameEvents, buildOrder)
			
			case *repcmd.LiftOffCmd:
				// 건물 이륙 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Lift Off at (%d, %d)", action.Pos.X, action.Pos.Y),
					"type":   "LiftOff",
				}
				gameEvents = append(gameEvents, buildOrder)
			
			case *repcmd.ChatCmd:
				// 채팅 이벤트
				buildOrder := gin.H{
					"time":   fmt.Sprintf("%d:%02d", action.BaseCmd().Frame/1500, (action.BaseCmd().Frame/25)%60),
					"player": replay.Header.PIDPlayers[action.BaseCmd().PlayerID].Name,
					"action": fmt.Sprintf("Chat: %s", action.Message),
					"type":   "Chat",
				}
				gameEvents = append(gameEvents, buildOrder)
			}
			
		}

		// 분석 결과 반환
		result := gin.H{
			"gameVersion": replay.Header.Version,
			"mapName":     replay.Header.Map,
			"players":     replay.Header.Players,
			"gameEvents": gameEvents,
		}

		c.JSON(http.StatusOK, result)
	})

	// 서버 실행
	r.Run(":9090")
}
