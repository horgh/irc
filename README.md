This is an IRC client library written in Go.

My intention is to be able to write IRC bots in Go. I've done so in Tcl and
Perl in the past, but these days I like to write Go.

It is pretty rough and basic and only supports what I need for my bots.
I'll add features as I need them.

Right now I have it set so you can create a package which add to irc.Hooks
via an init function, and the package will call your hook for every IRC
message. I define an IRC message as any line received from an IRC server.
This means you can create a package that takes action based on anything
that occurs on IRC.

Programs/packages:

  * duckduckgo: This is a package that causes the bot to query DuckDuckGo
    based on messages on IRC.
  * ircnotify: This is a small client that connects to IRC and joins a
    channel and sends a message, and then quits. It is useful if you need
    to notify an IRC channel from something like a cronjob.
  * test\_client: This is an example client. Currently it connects to IRC and
    acts like a bot.
