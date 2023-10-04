package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"github.com/kkdai/youtube/v2"
)

const (
	prefix string = "!bot"
	Folder string = "music"
)

var (
	client    *youtube.Client
	stop      chan bool
	direction string
)

func main() {

	client = &youtube.Client{}
	stop = make(chan bool)
	direction = ""

	godotenv.Load()
	token := os.Getenv("BOT_TOKEN")

	session, err := discordgo.New("Bot " + token)

	if err != nil {
		log.Fatal(err)
	}

	session.AddHandler(playAll)
	session.AddHandler(playURL)
	session.AddHandler(playPlaylist)
	session.AddHandler(nextTrack)

	session.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err = session.Open()
	if err != nil {
		log.Fatal(err)
	}

	//with defer this function is called when the program exits
	defer session.Close()

	fmt.Println("Bot is online")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func nextTrack(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	args := strings.Split(m.Content, " ")
	if args[0] != prefix {
		return
	}

	if args[1] == "next" {
		stop <- false
	}

	if args[1] == "prev" {
		direction = "prev"
		stop <- false
	}
}

func playAll(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	args := strings.Split(m.Content, " ")
	if args[0] != prefix {
		return
	}

	if args[1] == "play" && args[2] == "all" {

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			return
		}

		guild, err := s.State.Guild(channel.GuildID)
		if err != nil {
			return
		}

		channelID := ""
		for _, vs := range guild.VoiceStates {
			if vs.UserID == m.Author.ID {
				channelID = vs.ChannelID
				break
			}
		}

		vc, err := s.ChannelVoiceJoin(guild.ID, channelID, false, true)
		if err != nil {
			return
		}

		files, err := ioutil.ReadDir(Folder)
		if err != nil {
			return
		}

		for _, f := range files {
			fmt.Println("Play:", f.Name())
			dgvoice.PlayAudioFile(vc, fmt.Sprintf("%s/%s", Folder, f.Name()), make(chan bool))
		}

		vc.Close()
		vc.Disconnect()
	}
}

func playURL(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	args := strings.Split(m.Content, " ")
	if args[0] != prefix {
		return
	}

	if args[1] == "play" && args[2] != "all" {

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			return
		}

		guild, err := s.State.Guild(channel.GuildID)
		if err != nil {
			return
		}

		channelID := ""
		for _, vs := range guild.VoiceStates {
			if vs.UserID == m.Author.ID {
				channelID = vs.ChannelID
				break
			}
		}

		vc, _ := s.ChannelVoiceJoin(guild.ID, channelID, false, true)
		files, _ := ioutil.ReadDir(Folder)

		video, err := client.GetVideo(args[2])

		if err != nil {
			return
		}

		for _, f := range files {
			if f.Name() == video.ID+".mp4" {
				fmt.Println("PlayAudioFile:", f.Name())
				dgvoice.PlayAudioFile(vc, fmt.Sprintf("%s/%s", Folder, f.Name()), make(chan bool))
				vc.Close()
				vc.Disconnect()
				return
			}
		}

		formats := video.Formats.WithAudioChannels()
		url, _, err := client.GetStream(video, &formats[0])
		check(err)

		file, err := os.Create("music/" + video.ID + ".mp4")

		if err != nil {
			panic(err)
		}

		defer file.Close()
		_, err = io.Copy(file, url)
		if err != nil {
			panic(err)
		}

		fmt.Println("PlayAudioFile:", file.Name())
		dgvoice.PlayAudioFile(vc, file.Name(), make(chan bool))

		vc.Close()
		vc.Disconnect()
	}
}

func playPlaylist(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	args := strings.Split(m.Content, " ")
	if args[0] != prefix {
		return
	}

	if args[1] == "playlist" {

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			return
		}

		guild, err := s.State.Guild(channel.GuildID)
		if err != nil {
			return
		}

		channelID := ""
		for _, vs := range guild.VoiceStates {
			if vs.UserID == m.Author.ID {
				channelID = vs.ChannelID
				break
			}
		}

		vc, err := s.ChannelVoiceJoin(guild.ID, channelID, false, true)
		if err != nil {
			return
		}

		files, _ := ioutil.ReadDir(Folder)

		playlist, err := client.GetPlaylist(args[2])
		if err != nil {
			return
		}

		for i := 0; i < len(playlist.Videos); i++ {
			v := playlist.Videos[i]
			video, err := client.VideoFromPlaylistEntry(v)
			if err != nil {
				return
			}

			b := false
			for _, f := range files {
				if f.Name() == video.ID+".mp4" {
					fmt.Println("PlayAudioFile:", f.Name())
					dgvoice.PlayAudioFile(vc, fmt.Sprintf("%s/%s", Folder, f.Name()), stop)
					b = true
					if direction == "prev" {
						i = int(math.Min(float64(-1), float64(i-2)))
						direction = ""
					}
					break
				}
			}

			if b {
				continue
			}
			formats := video.Formats.WithAudioChannels()
			url, _, err := client.GetStream(video, &formats[0])
			check(err)

			file, err := os.Create("music/" + video.ID + ".mp4")

			if err != nil {
				panic(err)
			}

			defer file.Close()
			_, err = io.Copy(file, url)
			if err != nil {
				panic(err)
			}

			fmt.Println("PlayAudioFile:", file.Name())
			dgvoice.PlayAudioFile(vc, file.Name(), stop)

			if direction == "prev" {
				i = i - 2
				direction = ""
			}

		}

		vc.Close()
		vc.Disconnect()

	}
}
