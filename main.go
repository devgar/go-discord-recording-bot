package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/joho/godotenv/autoload"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

var (
	Start     uint32 = 0
	Token     string
	ChannelID string
	GuildID   string
	Delta     uint
	UsersMap  = map[int]string{}
)

func init() {
	UsersMap = map[int]string{}
	Token = os.Getenv("BOT_TOKEN")
	GuildID = os.Getenv("GUILD_ID")
	ChannelID = os.Getenv("CHANNEL_ID")

	flag.StringVar(&Token, "t", Token, "Bot Token")
	flag.StringVar(&GuildID, "g", GuildID, "Guild in which voice channel exists")
	flag.StringVar(&ChannelID, "c", ChannelID, "Voice channel to connect to")
	flag.UintVar(&Delta, "d", 20, "Duration of bot recording")
	flag.Parse()
}

func createPionRTPPacket(p *discordgo.Packet) *rtp.Packet {
	return &rtp.Packet{
		Header: rtp.Header{
			Version: 2,
			// Taken from Discord voice docs
			PayloadType:    0x78,
			SequenceNumber: p.Sequence,
			Timestamp:      p.Timestamp,
			SSRC:           p.SSRC,
		},
		Payload: p.Opus,
	}
}
func handleConn(v *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
	UsersMap[vs.SSRC] = vs.UserID
}

func parseMap() map[string]string {
	m := map[string]string{}
	fmt.Printf("\"__START_\": %d\n", Start)
	for key, value := range UsersMap {
		m[fmt.Sprintf("%d", key)] = value
		fmt.Printf("\"%d\": \"%s\"\n", key, value)
	}
	return m
}

func handleVoice(c chan *discordgo.Packet) {
	files := make(map[uint32]media.Writer)
	for p := range c {
		file, ok := files[p.SSRC]
		if !ok {
			var err error
			if Start == 0 {
				Start = p.Timestamp
			}
			file, err = oggwriter.New(fmt.Sprintf("%d_%d.ogg", p.SSRC, p.Timestamp), 48000, 2)
			if err != nil {
				fmt.Printf("failed to create file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
				return
			}
			files[p.SSRC] = file
		}
		fmt.Println(p.Timestamp)
		// Construct pion RTP packet from DiscordGo's type.
		rtp := createPionRTPPacket(p)
		err := file.WriteRTP(rtp)
		if err != nil {
			fmt.Printf("failed to write to file %d.ogg, giving up on recording: %v\n", p.SSRC, err)
		}
	}

	// Once we made it here, we're done listening for packets. Close all files
	for _, f := range files {
		f.Close()
	}
}

func main() {
	s, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session:", err)
		return
	}
	defer s.Close()

	// We only really care about receiving voice state updates.
	s.Identify.Intents = discordgo.IntentsGuildVoiceStates

	err = s.Open()
	if err != nil {
		fmt.Println("error opening connection:", err)
		return
	}

	// g, err := s.Guild(GuildID)
	// if err != nil {
	// 	fmt.Println("failed to get guild info:", err)
	// }

	v, err := s.ChannelVoiceJoin(GuildID, ChannelID, true, false)
	if err != nil {
		fmt.Println("failed to join voice channel:", err)
		return
	}

	v.AddHandler(handleConn)

	go func() {
		time.Sleep(time.Duration(Delta) * time.Second)
		close(v.OpusRecv)
		parseMap()
		v.Close()
	}()
	handleVoice(v.OpusRecv)
	fmt.Println("END OF FILE")
}
