package main

import (
	"bitbucket.org/tampajohn/gadget-arm/session"
	"fmt"
	"github.com/codegangsta/negroni"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"os"
	"strings"
	"bytes"
)

type Mention struct {
	Id     string `bson:"_id"`
	TeamId string `bson:"team_id"`
	Votes  int64  `bson:"votes"`
}

func VoteHandler(rw http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(5120)
	isValid := len(r.Form["text"]) > 0 && len(r.Form["team_id"]) > 0
	if !isValid {
		rw.Write([]byte("See ya homies"))
		return
	}
	isCmd := len(r.Form["command"]) > 0
	isTrg := len(r.Form["trigger_word"]) > 0
	db := session.Get("SLACKVOTE_DB").DB("slack-upvote")
	mentionId := r.Form["text"][0]
	teamId := r.Form["team_id"][0]
	sfx := ""

	if isTrg {
		mentionId = strings.TrimLeft(mentionId, r.Form["trigger_word"][0])
	}
	mentionId = strings.Trim(mentionId, " '\"")

	if mentionId == "" {
		rw.Write([]byte(""))
		return
	}
	isLoserBoard := mentionId == "-"
	isLeaderBoard := mentionId == "+"

	if !isLeaderBoard && !isLoserBoard {
		m := Mention{
			Id:     strings.ToLower(mentionId),
			TeamId: teamId,
		}
		db.C("mentions").Find(bson.M{"_id": m.Id, "team_id": m.TeamId}).One(&m)
		if isCmd {
			cmd := r.Form["command"][0]
			switch cmd {
			case "/up":
				m.Votes++
			case "/down":
				m.Votes--
			}
		} else if isTrg {
			trg := r.Form["trigger_word"][0]
			switch trg {
			case "+":
				m.Votes++
			case "-":
				m.Votes--
			}
		}
		db.C("mentions").UpsertId(m.Id, m)
		if m.Votes > 1 || m.Votes < -1 || m.Votes == 0 {
			sfx = "s"
		}
		text := fmt.Sprintf("'%s' has %v vote%s.", mentionId, m.Votes, sfx)
		if isCmd {
			rw.Write([]byte(text))
		} else if isTrg {
			rw.Write([]byte(fmt.Sprintf("{\"text\":\"%s\"}", text)))
		}
	} else {
		var sort string
		var title string
		var sb bytes.Buffer
		var fb bytes.Buffer
		if isLeaderBoard {
			sb.WriteString(fmt.Sprintf("Leader Board: "))
			sort = "-votes"
			title = "Leader Board"
		} else {
			sb.WriteString(fmt.Sprintf("Loser Board: "))
			sort = "votes"
			title = "Loser Board"
		}
		iter := db.C("mentions").Find(nil).Limit(10).Sort(sort).Iter()
		var m Mention
		for iter.Next(&m) {
			sb.WriteString(fmt.Sprintf("[*'%s'* has a score of *%v*], ", m.Id, m.Votes))
			fb.WriteString(fmt.Sprintf("'%s' has a score of %v", m.Id, m.Votes))
		}
		rw.Write([]byte(fmt.Sprintf("{\"text\":\"%s\", \"mkdown\":true, \"attachments\":[{\"fallback\":\"%s\", \"fields\" : [{ \"title\":\"%s\", \"value\":\"%s\"}] }] }", sb.String(), sb.String(), title, fb.String())))
	}
}
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", VoteHandler)

	n := negroni.New(
		negroni.NewLogger(),
		negroni.Wrap(mux),
	)
	n.Run(":" + os.Getenv("PORT"))
}
