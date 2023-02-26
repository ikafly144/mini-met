package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	discord "github.com/bwmarrin/discordgo"
)

type printMode int

const (
	modeErr printMode = iota
	modeInfo
	modeWarn
	modeDebug
)

func printe(mode printMode, label string, content string) {
	var modeStr string
	switch mode {
	case modeErr:
		modeStr = "ERROR"
	case modeInfo:
		modeStr = "INFO"
	case modeWarn:
		modeStr = "WARN"
	case modeDebug:
		modeStr = "DEBUG"
	default:
		modeStr = "UNKNOWN"
	}
	var labelStr string
	if label != "" {
		labelStr = fmt.Sprintf("[%s]", label)
	}
	fmt.Printf("[%s%s] %s\n", labelStr, modeStr, content)
}

// types

type GlobalChatData struct {
}

// variables

var intents = discord.IntentsAll
var session *discord.Session

var (
	globalChatDataRaw *os.File
	globalChatData    GlobalChatData
	blockedWords      []string
	tempData          = map[string]string{
		"acted_blocked_word_message_id": "",
		"reacted_message_id":            "",
		"received_dm_user_id":           "776726560929480707",
	}
	LastActedTimes = map[string]time.Time{
		"dayone_msg": time.Now(),
	}

	adminIDs = []string{
		"776726560929480707",
		"967372572859695184",
		"632596386772287532",
		"661416929168457739",
		"796350579286867988",
		"775952326493863936",
		"628513445964414997",
		"839884489424502855",
		"964438295440396320",
		"895267282413039646",
		"527514813799333889",
		"891337046239625306",
	}

	botToken      string
	metsServerID  = "842320961033601044"
	logChannelIDs = map[string]string{
		"member_joining_leaving": "1074249512605986836",
		"message_events":         "1074249514065596446",
		"member_events":          "1074249515554582548",
		"bot_log":                "1074249516871602227",
		"server_events":          "1074249522215137290",
		"auto_moderation":        "1074249523423105035",
		"voice_events":           "1074249525117603860",
	}

	mentionRegex = regexp.MustCompile(`<@[0-9]{18,20}>`)
	weatherRegex = regexp.MustCompile(`(明日の)(.{2,6})(の天気教えて)`)
)

func init() {
	printe(modeInfo, "INIT", "Loading...")
	var err error
	session, err = discord.New("Bot " + botToken)
	if err != nil {
		panic(err)
	}
	session.AddHandler(func(s *discord.Session, r *discord.Ready) {
		printe(modeInfo, "EVENT", fmt.Sprintf("%s is ready!!!", s.State.User.Username))
	})
	session.AddHandler(func(s *discord.Session, m *discord.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		if m.GuildID == "" {
			switch {
			case strings.HasPrefix(m.Content, "!sc "):
				cmd := strings.Trim(m.Content, "!sc ")
				var isAdmin bool
				for _, aID := range adminIDs {
					if m.Author.ID == aID {
						isAdmin = true
						break
					}
				}
				if !isAdmin {
					return
				}
				args := strings.Split(cmd, " ")
				if len(args) == 0 {
					return
				}
				switch args[0] {
				case "bword":
					if len(args) == 1 {
						return
					}
					args = args[1:]
					bwordBuf, err := os.ReadFile("storage/json/block_words.json")
					if err != nil {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Appending blocked word Exception in DMChannel:\r%s", err))
					}
					var bwordData []string
					err = json.Unmarshal(bwordBuf, &bwordData)
					if err != nil {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Appending blocked word Exception in DMChannel:\r%s", err))
					}
					if len(args) == 0 {
						return
					}
					switch args[0] {
					case "add":
						if len(args) == 1 {
							return
						}
						args = args[1:]
						bwordData = append(bwordData, args...)
					case "remove":
						if len(args) == 1 {
							return
						}
						args = args[1:]
						for _, v := range args {
							for i, v2 := range bwordData {
								if v2 == v {
									bwordData = bwordData[:i+copy(bwordData[i:], bwordData[i+1:])]
								}
							}
						}
					}
					blockedWords = bwordData
					bwordBuf, err = json.Marshal(bwordData)
					if err != nil {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Appending blocked word Exception in DMChannel:\r%s", err))
					}
					err = os.WriteFile("storage/json/block_words.json", bwordBuf, os.ModeTemporary)
					if err != nil {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Appending blocked word Exception in DMChannel:\r%s", err))
					}
				}
				return
			case m.Author.ID != tempData["received_dm_user_id"]:
				s.ChannelMessageSendEmbed("1065610631618764821", &discord.MessageEmbed{
					Author: &discord.MessageEmbedAuthor{
						Name:    fmt.Sprintf("DMの送信先が%sに変更されました", m.Author.Username+"#"+m.Author.Discriminator),
						IconURL: m.Author.AvatarURL(""),
					},
				})
				s.ChannelMessageSendEmbed("1065610631618764821", &discord.MessageEmbed{
					Description: m.Content,
					Author: &discord.MessageEmbedAuthor{
						Name:    m.Author.Username + "#" + m.Author.Discriminator,
						IconURL: m.Author.AvatarURL(""),
					},
					Footer: &discord.MessageEmbedFooter{
						Text: fmt.Sprintf("At: %s, Connecting ID: %s", time.Now().Format(time.RFC3339), m.Author.ID),
					},
				})
				tempData["received_dm_user_id"] = m.Author.ID
				return
			}
		}
		switch {
		case m.Author.ID != s.State.User.ID && m.GuildID == metsServerID:
			for _, v := range blockedWords {
				if strings.Contains(m.Content, v) {
					printe(modeInfo, "BWORD", fmt.Sprintf("Detected blocked word: %s", m.Content))
					embed := &discord.MessageEmbed{
						Title:       "ワードフィルタにかかるメッセージを検知しました",
						URL:         fmt.Sprintf("https://discord.com/channels/%s/%s/%s", m.GuildID, m.ChannelID, m.ID),
						Description: fmt.Sprintf("チャンネル: <#%s>\rユーザー: <@%s>", m.ChannelID, m.Author.ID),
						Author: &discord.MessageEmbedAuthor{
							IconURL: m.Author.AvatarURL(""),
							Name:    m.Author.Username + "#" + m.Author.Discriminator,
						},
						Footer: &discord.MessageEmbedFooter{
							Text: fmt.Sprintf("MID: %s ,ChID: %s At: %s", m.ID, m.ChannelID, time.Now().Format(time.RFC3339)),
						},
						Fields: []*discord.MessageEmbedField{
							{
								Name:  "メッセージ",
								Value: m.Content,
							},
						},
					}
					s.ChannelMessageSendEmbed(logChannelIDs["auto_moderation"], embed)
					s.MessageReactionAdd(m.ChannelID, m.ID, "❗")
					tempData["acted_blocked_word_message_id"] = m.ID
					time.Sleep(time.Second * 5)
					s.MessageReactionRemove(m.ChannelID, m.ID, "❗", s.State.User.ID)
				}
			}
		case m.Author.Bot:
			return
		case mentionRegex.MatchString(m.Content):
			printe(modeInfo, "MENTIONLOG", "Received ")
			s.ChannelMessageSendEmbed(logChannelIDs["message_events"], &discord.MessageEmbed{
				Title: "mention message log",
				URL:   fmt.Sprintf("https://discord.com/channels/%s/%s/%s", m.GuildID, m.ChannelID, m.ID),
				Fields: []*discord.MessageEmbedField{
					{
						Name:  "content:",
						Value: m.Content,
					},
				},
			})
		case strings.HasPrefix(m.Content, "<@985254515798327296>"):
			command, found := strings.CutPrefix(m.Content, "<@985254515798327296> ")
			if !found {
				command = strings.TrimPrefix(m.Content, "<@985254515798327296>")
			}
			printe(modeInfo, "MENTIONCMD", fmt.Sprintf("Received Mention Command: %s", command))
			time.Sleep(time.Second * 2)

			switch {
			case command == "さいころ振って":
				s.ChannelTyping(m.ChannelID)
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%d!", rand.Intn(6)))
				return
			case command == "今のドル円教えて":
				s.ChannelTyping(m.ChannelID)
				res, err := http.Get("https://www.gaitameonline.com/rateaj/getrate")
				if err != nil {
					return
				}
				buf, err := io.ReadAll(res.Body)
				if err != nil {
					return
				}
				rate := map[string]any{}
				json.Unmarshal(buf, &rate)
				rateMap, ok := rate["quotes"].([]any)[20].(map[string]string)
				if !ok {
					return
				}
				s.ChannelMessageSendEmbed(m.ChannelID, &discord.MessageEmbed{
					Title:       rateMap["currencyPairCode"],
					Description: fmt.Sprintf("High: %s\rLow: %s", rateMap["high"], rateMap["low"]),
				})
				return
			case weatherRegex.MatchString(m.Content):
				command = strings.TrimLeft(command, "明日の")
				command = strings.TrimRight(command, "の天気教えて")
				switch {
				case strings.HasSuffix(command, "地方"):
					command = strings.TrimRight(command, "地方")
				case strings.HasSuffix(command, "県"):
					command = strings.TrimRight(command, "県")
				case strings.HasSuffix(command, "都"):
					command = strings.TrimRight(command, "都")
				case strings.HasSuffix(command, "府"):
					command = strings.TrimRight(command, "府")
				}
				var areaCode string
				switch command {
				case "宗谷":
					areaCode = "011000"
				case "上川":
					areaCode = "012000"
				case "留萌":
					areaCode = "012000"
				case "網走":
					areaCode = "013000"
				case "北見":
					areaCode = "013000"
				case "紋別":
					areaCode = "013000"
				case "十勝":
					areaCode = "014030"
				case "釧路":
					areaCode = "014100"
				case "根室":
					areaCode = "014100"
				case "胆振":
					areaCode = "015000"
				case "日高":
					areaCode = "015000"
				case "石狩":
					areaCode = "016000"
				case "空知":
					areaCode = "016000"
				case "後志":
					areaCode = "016000"
				case "渡島":
					areaCode = "017000"
				case "檜山":
					areaCode = "017000"
				case "青森":
					areaCode = "020000"
				case "岩手":
					areaCode = "030000"
				case "宮城":
					areaCode = "040000"
				case "秋田":
					areaCode = "050000"
				case "山形":
					areaCode = "060000"
				case "福島":
					areaCode = "070000"
				case "茨城":
					areaCode = "080000"
				case "栃木":
					areaCode = "090000"
				case "群馬":
					areaCode = "100000"
				case "埼玉":
					areaCode = "110000"
				case "千葉":
					areaCode = "120000"
				case "東京":
					areaCode = "130000"
				case "神奈川":
					areaCode = "140000"
				case "山梨":
					areaCode = "190000"
				case "長野":
					areaCode = "200000"
				case "岐阜":
					areaCode = "210000"
				case "静岡":
					areaCode = "220000"
				case "愛知":
					areaCode = "230000"
				case "三重":
					areaCode = "240000"
				case "新潟":
					areaCode = "150000"
				case "富山":
					areaCode = "160000"
				case "石川":
					areaCode = "170000"
				case "福井":
					areaCode = "180000"
				case "滋賀":
					areaCode = "250000"
				case "京都":
					areaCode = "260000"
				case "大阪":
					areaCode = "270000"
				case "兵庫":
					areaCode = "280000"
				case "奈良":
					areaCode = "290000"
				case "和歌山":
					areaCode = "300000"
				case "鳥取":
					areaCode = "310000"
				case "島根":
					areaCode = "320000"
				case "岡山":
					areaCode = "330000"
				case "広島":
					areaCode = "340000"
				case "徳島":
					areaCode = "360000"
				case "香川":
					areaCode = "370000"
				case "愛媛":
					areaCode = "380000"
				case "高知":
					areaCode = "390000"
				case "山口":
					areaCode = "350000"
				case "福岡":
					areaCode = "400000"
				case "佐賀":
					areaCode = "410000"
				case "長崎":
					areaCode = "420000"
				case "熊本":
					areaCode = "430000"
				case "大分":
					areaCode = "440000"
				case "宮崎":
					areaCode = "450000"
				case "奄美":
					areaCode = "460040"
				case "鹿児島":
					areaCode = "460100"
				case "沖縄本島":
					areaCode = "471000"
				case "大東島":
					areaCode = "472000"
				case "宮古島":
					areaCode = "473000"
				case "八重山":
					areaCode = "474000"
				default:
					areaCode = "Not Found"
				}
				if areaCode == "Not Found" {
					s.ChannelMessageSend(m.ChannelID, "しらん")
					return
				}
				res, err := http.Get(fmt.Sprintf("https://www.jma.go.jp/bosai/forecast/data/forecast/%s.json", areaCode))
				if err != nil {
					printe(modeErr, "", fmt.Sprintf("Error at get weather data: %s", err))
				}
				buf, err := io.ReadAll(res.Body)
				if err != nil {
					printe(modeErr, "", fmt.Sprintf("Error at get weather data: %s", err))
				}
				data := []map[string]any{}
				json.Unmarshal(buf, &data)
				embed := []*discord.MessageEmbed{
					{
						Title:       "天気by気象庁",
						URL:         "https://www.jma.go.jp/jma/",
						Description: fmt.Sprintf("%s | %s", data[0]["publishingOffice"], data[0]["reportDatetime"]),
						Color:       0xCCCCCC,
					},
				}
				t, _ := time.Parse("2006-01-02T15:04:05+07:00", data[0]["timeSeries"].([]map[string]any)[0]["timeDefines"].([]string)[1])
				embed = append(embed, &discord.MessageEmbed{
					Title:       "天気",
					Description: fmt.Sprintf("<t:%d:F>", t.Unix()),
					Color:       0xFF8888,
				})
			}
		}
	})
}
