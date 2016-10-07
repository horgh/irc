This is a collection of IRC programs and libraries written in Go. It includes a
library for working with the IRC protocol (decoding/encoding). There is also a
client library and packages that add functionality to the client.

The repository also includes sample clients using the library.


# Packages

## irc
This package provides functionality for working with the IRC protocol. Specifically,
it allows you to decode and encode IRC messages.


## irc/client
This package is an IRC client library.

My intention is to be able to write IRC bots in Go. I've done so in Tcl and
Perl in the past, but these days I like to write Go.

It is basic and only supports what I need for my bots. I add features as I need
them.

You can create a package which add to client.Hooks via an init function, and
the package will call your hook for every IRC message. I define an IRC message
as any line received from an IRC server. This means you can create a package
that takes action based on anything that occurs on IRC. I intend it as a way to
"script" bots.


### Client package: irc/duckduckgo
This is a package that causes an IRC client to act as a bot and to query
DuckDuckGo based on messages on IRC.


### Client package: irc/oper
This package makes a client become an IRC operator. You need to define
`oper-name` and `oper-password` in your client's configuration to use it.


### Client package: irc/recordips
This package causes a client that is an IRC operator to record connecting IPs
to a file. It's based on ircd-ratbox notices.


## Client: irc/ircnotify
This is a small client that connects to IRC, joins a channel, sends a message,
and then quits. It is useful if you need to notify an IRC channel from
something like a cronjob.


### Client: irc/test\_client
This is an example client. Currently it connects to IRC and acts like a bot. It
demonstrates using several of the client packages.
