package main

import (
	"github.com/jonas747/cardsagainstdiscord"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/dstate"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

var cahManager *cardsagainstdiscord.GameManager

func panicErr(err error, msg string) {
	if err != nil {
		panic(msg + ": " + err.Error())
	}
}

func main() {
	session, err := discordgo.New(os.Getenv("DG_TOKEN"))
	panicErr(err, "Failed initializing discordgo")

	cahManager = cardsagainstdiscord.NewGameManager(&cardsagainstdiscord.StaticSessionProvider{
		Session: session,
	})

	state := dstate.NewState()
	state.TrackMembers = false
	state.TrackPresences = false
	session.StateEnabled = false

	cmdSys := dcmd.NewStandardSystem("!cah")
	cmdSys.State = state
	cmdSys.Root.AddCommand(CreateGameCommand, dcmd.NewTrigger("create", "c").SetDisableInDM(true))
	cmdSys.Root.AddCommand(StopCommand, dcmd.NewTrigger("stop", "end", "s").SetDisableInDM(true))
	cmdSys.Root.AddCommand(KickCommand, dcmd.NewTrigger("kick").SetDisableInDM(true))

	session.AddHandler(state.HandleEvent)
	session.AddHandler(cmdSys.HandleMessageCreate)
	session.AddHandler(func(s *discordgo.Session, ra *discordgo.MessageReactionAdd) {
		go cahManager.HandleReactionAdd(ra)
	})

	session.AddHandler(func(s *discordgo.Session, msg *discordgo.MessageCreate) {
		go cahManager.HandleMessageCreate(msg)
	})

	err = session.Open()
	panicErr(err, "Failed opening gateway connection")
	log.Println("Running...")

	// We import http/pprof above to be ale to inspect shizz and do profiling
	go http.ListenAndServe(":7447", nil)
	select {}
}

var CreateGameCommand = &dcmd.SimpleCmd{
	ShortDesc: "Creates a cards against humanity game in this channel",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		_, err := cahManager.CreateGame(data.GS.ID, data.CS.ID, data.Msg.Author.ID, data.Msg.Author.Username, "main")
		if err == nil {
			log.Println("Created a new game in ", data.CS.ID)
			return "", nil
		}

		if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
			return cahErr, nil
		}

		return "Something went wrong", err

	},
}

var StopCommand = &dcmd.SimpleCmd{
	ShortDesc: "Stops a cards against humanity game in this channel",
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		game := cahManager.FindGameFromChannelOrUser(data.Msg.Author.ID)
		if game == nil {
			return "Couln't find any game you're part of", nil
		}

		if game.GameMaster != data.Msg.Author.ID {
			return "You're not the game master of this game", nil
		}

		err := cahManager.RemoveGame(data.CS.ID)
		if err != nil {
			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "Something went wrong", err
		}

		return "Stopped the game", nil
	},
}

var KickCommand = &dcmd.SimpleCmd{
	ShortDesc:       "Kicks a player from the card against humanity game in this channle, only the game master can do this",
	RequiredArgDefs: 1,
	CmdArgDefs: []*dcmd.ArgDef{
		&dcmd.ArgDef{Name: "user", Type: dcmd.UserID},
	},
	RunFunc: func(data *dcmd.Data) (interface{}, error) {
		game := cahManager.FindGameFromChannelOrUser(data.Msg.Author.ID)
		if game == nil {
			return "Couln't find any game you're part of", nil
		}

		if game.GameMaster != data.Msg.Author.ID {
			return "You're not the game master of this game", nil
		}

		userID := data.Args[0].Int64()
		game.RLock()
		found := false
		for _, v := range game.Players {
			if v.ID == userID {
				found = true
				break
			}
		}
		game.RUnlock()

		if !found {
			return "User is not part of your game", nil
		}

		err := cahManager.PlayerTryLeaveGame(userID)
		if err != nil {
			if cahErr := cardsagainstdiscord.HumanizeError(err); cahErr != "" {
				return cahErr, nil
			}

			return "Something went wrong", err
		}

		return "User removed", nil
	},
}
