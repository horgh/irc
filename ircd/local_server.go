package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"summercat.com/irc"
)

// LocalServer means the client registered as a server. This holds its info.
type LocalServer struct {
	*LocalClient

	Server *Server

	Capabs map[string]struct{}

	// The last time we heard anything from it.
	LastActivityTime time.Time

	// The last time we sent it a PING.
	LastPingTime time.Time

	// Flags to know about our bursting state.
	GotPING  bool
	GotPONG  bool
	Bursting bool
}

// NewLocalServer upgrades a LocalClient to a LocalServer.
func NewLocalServer(c *LocalClient) *LocalServer {
	now := time.Now()

	s := &LocalServer{
		LocalClient:      c,
		Capabs:           c.PreRegCapabs,
		LastActivityTime: now,
		LastPingTime:     now,
		GotPING:          false,
		GotPONG:          false,
		Bursting:         true,
	}

	return s
}

func (s *LocalServer) String() string {
	return s.Server.String()
}

func (s *LocalServer) getLastActivityTime() time.Time {
	return s.LastActivityTime
}

func (s *LocalServer) getLastPingTime() time.Time {
	return s.LastPingTime
}

func (s *LocalServer) setLastPingTime(t time.Time) {
	s.LastPingTime = t
}

func (s *LocalServer) messageFromServer(command string, params []string) {
	// For numeric messages, we need to prepend the nick.
	// Use * for the nick in cases where the client doesn't have one yet.
	// This is what ircd-ratbox does. Maybe not RFC...
	if isNumericCommand(command) {
		newParams := []string{string(s.Server.SID)}
		newParams = append(newParams, params...)
		params = newParams
	}

	s.maybeQueueMessage(irc.Message{
		Prefix:  s.Catbox.Config.TS6SID,
		Command: command,
		Params:  params,
	})
}

func (s *LocalServer) quit(msg string) {
	// May already be cleaning up.
	_, exists := s.Catbox.LocalServers[s.ID]
	if !exists {
		return
	}

	s.messageFromServer("ERROR", []string{msg})

	close(s.WriteChan)

	delete(s.Catbox.LocalServers, s.ID)
	delete(s.Catbox.Servers, s.Server.SID)

	// TODO: Make all clients quit that are on the other side.

	// TODO: Inform any other servers that are connected.
}

// Send the burst. This tells the server about the state of the world as we see
// it.
// We send our burst after seeing SVINFO. This means we have not yet processed
// any SID, UID, or SJOIN messages from the other side.
func (s *LocalServer) sendBurst() {
	// Send all our connected servers with SID commands.
	// Parameters: <server name> <hop count> <SID> <description>
	// e.g.: :8ZZ SID irc3.example.com 2 9ZQ :My Desc
	for _, server := range s.Catbox.Servers {
		// Don't send it itself.
		if server.LocalServer == s {
			continue
		}

		s.maybeQueueMessage(irc.Message{
			Prefix:  s.Catbox.Config.TS6SID,
			Command: "SID",
			Params: []string{
				server.Name,
				fmt.Sprintf("%d", server.HopCount+1),
				string(server.SID),
				server.Description,
			},
		})
	}

	// Send all our users with UID commands.
	// Parameters: <nick> <hopcount> <nick TS> <umodes> <username> <hostname> <IP> <UID> :<real name>
	// :8ZZ UID will 1 1475024621 +i will blashyrkh. 0 8ZZAAAAAB :will
	for _, user := range s.Catbox.Users {
		s.maybeQueueMessage(irc.Message{
			Prefix:  s.Catbox.Config.TS6SID,
			Command: "UID",
			Params: []string{
				user.DisplayNick,
				// Hop count increases for them.
				fmt.Sprintf("%d", user.HopCount+1),
				fmt.Sprintf("%d", user.NickTS),
				user.modesString(),
				user.Username,
				user.Hostname,
				user.IP,
				string(user.UID),
				user.RealName,
			},
		})
	}

	// Send channels and the users in them with SJOIN commands.
	// Parameters: <channel TS> <channel name> <modes> [mode params] :<UIDs>
	// e.g., :8ZZ SJOIN 1475187553 #test2 +sn :@8ZZAAAAAB
	for _, channel := range s.Catbox.Channels {
		// TODO: Combine as many UIDs into a single SJOIN as we can, rather than
		//   one SJOIN per UID.
		for uid := range channel.Members {
			s.maybeQueueMessage(irc.Message{
				Prefix:  s.Catbox.Config.TS6SID,
				Command: "SJOIN",
				Params: []string{
					fmt.Sprintf("%d", channel.TS),
					channel.Name,
					"+nt",
					string(uid),
				},
			})
		}
	}
}

func (s *LocalServer) sendPING() {
	// PING <My SID>
	s.maybeQueueMessage(irc.Message{
		Command: "PING",
		Params: []string{
			s.Catbox.Config.TS6SID,
		},
	})
}

func (s *LocalServer) handleMessage(m irc.Message) {
	// Record that client said something to us just now.
	s.LastActivityTime = time.Now()

	if m.Command == "PING" {
		s.pingCommand(m)
		return
	}

	if m.Command == "PONG" {
		s.pongCommand(m)
		return
	}

	if m.Command == "ERROR" {
		s.errorCommand(m)
		return
	}

	if m.Command == "UID" {
		s.uidCommand(m)
		return
	}

	if m.Command == "PRIVMSG" || m.Command == "NOTICE" {
		s.privmsgCommand(m)
		return
	}

	// For now I ignore ENCAP.
	if m.Command == "ENCAP" {
		return
	}

	if m.Command == "SID" {
		s.sidCommand(m)
		return
	}

	if m.Command == "SJOIN" {
		s.sjoinCommand(m)
		return
	}

	// 421 ERR_UNKNOWNCOMMAND
	s.messageFromServer("421", []string{m.Command, "Unknown command"})
}

func (s *LocalServer) pingCommand(m irc.Message) {
	// We expect a PING from server as part of burst end.
	// PING <Remote SID>
	if len(m.Params) < 1 {
		// 461 ERR_NEEDMOREPARAMS
		s.messageFromServer("461", []string{"PING", "Not enough parameters"})
		return
	}

	// Allow multiple pings.

	if len(m.Prefix) == 0 {
		m.Prefix = string(s.Server.SID)
	}

	// :9ZQ PING irc3.example.com :000
	// Where irc3.example.com == 9ZQ and it is remote

	// We want to send back
	// :000 PONG irc.example.com :9ZQ

	sid := TS6SID(m.Prefix)

	// Do we know the server pinging us?
	_, exists := s.Catbox.Servers[sid]
	if !exists {
		// 402 ERR_NOSUCHSERVER
		s.maybeQueueMessage(irc.Message{
			Prefix:  s.Catbox.Config.TS6SID,
			Command: "402",
			Params:  []string{string(sid), "No such server"},
		})
		return
	}

	// Reply.
	s.maybeQueueMessage(irc.Message{
		Prefix:  s.Catbox.Config.TS6SID,
		Command: "PONG",
		Params:  []string{s.Catbox.Config.ServerName, string(sid)},
	})

	// If we're bursting, is it over?
	if s.Bursting && sid == s.Server.SID {
		s.GotPING = true

		if s.GotPONG {
			s.Catbox.noticeOpers(fmt.Sprintf("Burst with %s over.", s.Server.Name))
			s.Bursting = false
		}
	}
}

func (s *LocalServer) pongCommand(m irc.Message) {
	// We expect this at end of server link burst.
	// :<Remote SID> PONG <Remote server name> <My SID>
	if len(m.Params) < 2 {
		// 461 ERR_NEEDMOREPARAMS
		s.messageFromServer("461", []string{"SVINFO", "Not enough parameters"})
		return
	}

	if TS6SID(m.Prefix) != s.Server.SID {
		s.quit("Unknown prefix")
		return
	}

	if m.Params[0] != s.Server.Name {
		s.quit("Unknown server name")
		return
	}

	if m.Params[1] != s.Catbox.Config.TS6SID {
		s.quit("Unknown SID")
		return
	}

	// No reply.

	s.GotPONG = true

	if s.Bursting && s.GotPING {
		s.Catbox.noticeOpers(fmt.Sprintf("Burst with %s over.", s.Server.Name))
		s.Bursting = false
	}
}

func (s *LocalServer) errorCommand(m irc.Message) {
	s.quit("Bye")
}

// UID command introduces a client. It is on the server that is the source.
func (s *LocalServer) uidCommand(m irc.Message) {
	// Parameters: <nick> <hopcount> <nick TS> <umodes> <username> <hostname> <IP> <UID> :<real name>
	// :8ZZ UID will 1 1475024621 +i will blashyrkh. 0 8ZZAAAAAB :will

	// Is this a valid SID (format)?
	if !isValidSID(m.Prefix) {
		s.quit("Invalid SID")
		return
	}
	sid := TS6SID(m.Prefix)

	// Do we know the server the message originates on?
	_, exists := s.Catbox.Servers[TS6SID(sid)]
	if !exists {
		s.quit("Message from unknown server")
		return
	}

	// Is this a valid nick?
	if !isValidNick(s.Catbox.Config.MaxNickLength, m.Params[0]) {
		s.quit("Invalid NICK!")
		return
	}
	displayNick := m.Params[0]

	// Is there a nick collision?
	_, exists = s.Catbox.Nicks[canonicalizeNick(displayNick)]
	if exists {
		// TODO: Issue kill(s). For now just kick the server.
		s.quit("Nick collision")
		return
	}

	hopCount, err := strconv.ParseInt(m.Params[1], 10, 8)
	if err != nil {
		s.quit("Invalid hop count")
		return
	}

	nickTS, err := strconv.ParseInt(m.Params[2], 10, 64)
	if err != nil {
		s.quit("Invalid nick TS")
		return
	}

	umodes := make(map[byte]struct{})
	for i, umode := range m.Params[3] {
		if i == 0 {
			if umode != '+' {
				s.quit("Malformed umode")
				return
			}
			continue
		}

		// I only support +i and +o right now.
		if umode == 'i' || umode == 'o' {
			umodes[byte(umode)] = struct{}{}
			continue
		}
	}

	if !isValidUser(s.Catbox.Config.MaxNickLength, m.Params[4]) {
		s.quit("Invalid username")
		return
	}
	username := m.Params[4]

	// TODO: Validate hostname
	hostname := m.Params[5]

	// TODO: Validate IP
	ip := m.Params[6]

	if !isValidUID(m.Params[7]) {
		s.quit("Invalid UID")
		return
	}
	uid := TS6UID(m.Params[7])

	if !isValidRealName(m.Params[8]) {
		s.quit("Invalid real name")
		return
	}
	realName := m.Params[8]

	// OK, the user looks good.

	u := &User{
		DisplayNick: displayNick,
		HopCount:    int(hopCount),
		NickTS:      int64(nickTS),
		Modes:       umodes,
		Username:    username,
		Hostname:    hostname,
		IP:          ip,
		UID:         uid,
		RealName:    realName,
		Channels:    make(map[string]*Channel),
		Link:        s,
	}

	if u.isOperator() {
		s.Catbox.Opers[u.UID] = u
	}
	s.Catbox.Nicks[canonicalizeNick(displayNick)] = u.UID
	s.Catbox.Users[u.UID] = u

	// No reply needed I think.
}

func (s *LocalServer) privmsgCommand(m irc.Message) {
	// Parameters: <msgtarget> <text to be sent>

	if len(m.Params) == 0 {
		// 411 ERR_NORECIPIENT
		s.messageFromServer("411", []string{"No recipient given (PRIVMSG)"})
		return
	}

	if len(m.Params) == 1 {
		// 412 ERR_NOTEXTTOSEND
		s.messageFromServer("412", []string{"No text to send"})
		return
	}

	// Do we recognize the source?
	if !isValidUID(m.Prefix) {
		s.quit("Invalid source")
		return
	}

	sourceUID := TS6UID(m.Prefix)

	sourceUser, exists := s.Catbox.Users[sourceUID]
	if !exists {
		s.quit("I don't know this source")
		return
	}

	// Is target a user?
	if isValidUID(m.Params[0]) {
		targetUID := TS6UID(m.Params[0])

		targetUser, exists := s.Catbox.Users[targetUID]
		if exists {
			// We either deliver it to a local user, and done, or we need to propagate
			// it to another server.
			if targetUser.isLocal() {
				// Source and target were UIDs. Translate to uhost and nick
				// respectively.
				m.Params[0] = targetUser.DisplayNick
				targetUser.LocalUser.maybeQueueMessage(irc.Message{
					Prefix:  sourceUser.nickUhost(),
					Command: m.Command,
					Params:  m.Params,
				})
			} else {
				// Propagate to the server we know the target user through.
				targetUser.Link.maybeQueueMessage(m)
			}

			return
		}

		// Fall through. Treat it as a channel name.
	}

	// TODO: Is target a channel?
}

// SID tells us about a new server.
func (s *LocalServer) sidCommand(m irc.Message) {
	// Parameters: <server name> <hop count> <SID> <description>
	// e.g.: :8ZZ SID irc3.example.com 2 9ZQ :My Desc

	if !isValidSID(m.Prefix) {
		s.quit("Invalid origin")
		return
	}
	originSID := TS6SID(m.Prefix)

	// Do I know this origin?
	_, exists := s.Catbox.Servers[originSID]
	if !exists {
		s.quit("Unknown origin")
		return
	}

	if len(m.Params) < 4 {
		// 461 ERR_NEEDMOREPARAMS
		s.messageFromServer("461", []string{"SID", "Not enough parameters"})
		return
	}

	name := m.Params[0]

	hopCount, err := strconv.ParseInt(m.Params[1], 10, 8)
	if err != nil {
		s.quit(fmt.Sprintf("Invalid hop count: %s", err))
		return
	}

	if !isValidSID(m.Params[2]) {
		s.quit("Invalid SID")
		return
	}
	sid := TS6SID(m.Params[2])

	desc := m.Params[3]

	newServer := &Server{
		SID:         sid,
		Name:        name,
		Description: desc,
		HopCount:    int(hopCount),
		Link:        s,
	}

	s.Catbox.Servers[sid] = newServer

	// Propagate to our connected servers.
	for _, server := range s.Catbox.LocalServers {
		// Don't tell the server we just heard it from.
		if server == s {
			continue
		}

		server.maybeQueueMessage(m)
	}
}

// SJOIN occurs in two contexts:
// 1. During bursts to inform us of channels and users in the channels.
// 2. Outside bursts to inform us of channel creation (not joins in general)
func (s *LocalServer) sjoinCommand(m irc.Message) {
	// Parameters: <channel TS> <channel name> <modes> [mode params] :<UIDs>
	// e.g., :8ZZ SJOIN 1475187553 #test2 +sn :@8ZZAAAAAB

	// Do we know this server?
	_, exists := s.Catbox.Servers[TS6SID(m.Prefix)]
	if !exists {
		s.quit("Unknown server")
		return
	}

	if len(m.Params) < 4 {
		// 461 ERR_NEEDMOREPARAMS
		s.messageFromServer("461", []string{"PING", "Not enough parameters"})
		return
	}

	channelTS, err := strconv.ParseInt(m.Params[0], 10, 64)
	if err != nil {
		s.quit(fmt.Sprintf("Invalid channel TS: %s: %s", m.Params[0], err))
		return
	}

	chanName := m.Params[1]

	// Currently I ignore modes. All channels have the same mode, or we pretend so
	// anyway.

	channel, exists := s.Catbox.Channels[canonicalizeChannel(chanName)]
	if !exists {
		channel = &Channel{
			Name:    canonicalizeChannel(chanName),
			Members: make(map[TS6UID]struct{}),
			TS:      channelTS,
		}
		s.Catbox.Channels[channel.Name] = channel
	}

	// If we had mode parameters, then user list is bumped up one.
	userList := m.Params[3]
	if len(m.Params) > 4 {
		userList = m.Params[4]
	}

	// Look at each of the members we were told about.
	uidsRaw := strings.Split(userList, " ")
	for _, uidRaw := range uidsRaw {
		// May have op/voice prefix.
		// Cut it off for now. I currently don't support op/voice.
		uidRaw = strings.TrimLeft(uidRaw, "@+")

		if !isValidUID(uidRaw) {
			s.quit("Invalid UID")
			// TODO: Possible to have empty channel at this point
			return
		}

		uid := TS6UID(uidRaw)

		user, exists := s.Catbox.Users[uid]
		if !exists {
			s.quit("Unknown user")
			// TODO: Possible to have empty channel at this point
			return
		}

		// We could check if we already have them flagged as in the channel.

		channel.Members[user.UID] = struct{}{}
		user.Channels[channel.Name] = channel
	}

	// Propagate.
	for _, server := range s.Catbox.LocalServers {
		// Don't send it to the server we just heard it from.
		if server == s {
			continue
		}

		server.maybeQueueMessage(m)
	}
}
