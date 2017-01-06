package irc

import (
	"testing"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		input   string
		prefix  string
		command string
		params  []string
		fail    bool
	}{
		{":irc PRIVMSG\r\n", "irc", "PRIVMSG", []string{}, false},

		{":irc PRIVMSG", "", "", []string{}, true},

		{":irc PRIVMSG one", "", "", []string{}, true},

		{":irc \r\n", "", "", []string{}, true},
		{"PRIVMSG\r\n", "", "PRIVMSG", []string{}, false},
		{"PRIVMSG :hi there\r\n", "", "PRIVMSG", []string{"hi there"}, false},
		{": PRIVMSG \r\n", "", "", []string{}, true},
		{"ir\rc\r\n", "", "", []string{}, true},

		{":irc PRIVMSG blah\r\n", "irc", "PRIVMSG", []string{"blah"}, false},
		{":irc 001 :Welcome\r\n", "irc", "001", []string{"Welcome"}, false},
		{":irc 001\r\n", "irc", "001", []string{}, false},

		// This is technically invalid per grammar as there is a trailing space.
		// However I permit it as we see trailing space in the wild frequently.
		{":irc PRIVMSG \r\n", "irc", "PRIVMSG", []string{}, false},

		{":irc @01\r\n", "", "", []string{}, true},
		{":irc \r\n", "", "", []string{}, true},
		{":irc  PRIVMSG\r\n", "", "", []string{}, true},

		{":irc 000 hi\r\n", "irc", "000", []string{"hi"}, false},

		// It is valid to have no parameters.
		{":irc 000\r\n", "irc", "000", []string{}, false},

		// Test last param having no :
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5\r\n", "irc", "000", []string{"1",
			"2", "3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", "5"},
			false},

		// Test last param having no : nor characters
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 \r\n", "irc", "000",
			[]string{"1", "2",
				"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", ""}, false},

		// Test last param having : but no characters
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 :\r\n", "irc", "000",
			[]string{"1", "2",
				"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", ""}, false},

		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 :hi there\r\n", "irc", "000",
			[]string{"1", "2",
				"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", "hi there"},
			false},

		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 hi there\r\n", "irc", "000",
			[]string{"1", "2",
				"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", "hi there"},
			false},

		// Malformed because \r should not appear there.
		{":irc 000 \r\r\n", "", "", []string{}, true},

		// Param must not be blank unless last param.
		// While this violates the grammar, I permit it now anyway.
		{":irc 000 \r\n", "irc", "000", []string{}, false},

		{":irc 000 0a 1b\r\n", "irc", "000", []string{"0a", "1b"}, false},

		// If we have a space then there must be a parameter (unless it's the
		// 15th).
		// While this violates the grammar, I permit it now anyway.
		{":irc 000 0 1 \r\n", "irc", "000", []string{"0", "1"}, false},

		{":irc 000 a\x00 1 \r\n", "", "", []string{}, true},

		// : inside a middle. Valid.
		{":irc 000 a:bc\r\n", "irc", "000", []string{"a:bc"}, false},

		{":irc 000 hi :there yes\r\n", "irc", "000", []string{"hi", "there yes"},
			false},

		// : inside a middle parameter. This is valid.
		{":irc 000 hi:hi :no no\r\n", "irc", "000", []string{"hi:hi", "no no"},
			false},

		{":irc 000 hi:hi :no no :yes yes\r\n", "irc", "000", []string{"hi:hi", "no no :yes yes"},
			false},

		{":irc 000 hi:hi :no no :yes yes\n", "irc", "000", []string{"hi:hi", "no no :yes yes"},
			false},

		// Fails and SHOULD actually. Trailing whitespace is not valid here.
		// Ratbox currently does send messages like this however.
		{":irc MODE #test +o user  \r\n", "irc", "MODE", []string{"+o", "user"},
			true},

		// Blank topic parameter is used to unset the topic.
		{":nick!user@host TOPIC #test :\r\n", "nick!user@host", "TOPIC", []string{"#test", ""},
			false},

		{":nick!user@host MODE #test +o :blah\r\n", "nick!user@host", "MODE",
			[]string{"#test", "+o", "blah"}, false},

		{":nick!user@host MODE #test +o blah1 :blah\r\n", "nick!user@host", "MODE",
			[]string{"#test", "+o", "blah1", "blah"}, false},

		{":nick!user@host MODE #test +o :blah1 blah\r\n", "nick!user@host", "MODE",
			[]string{"#test", "+o", "blah1 blah"}, false},
	}

	for _, test := range tests {
		msg, err := ParseMessage(test.input)
		if err != nil {
			if test.fail != true {
				t.Errorf("ParseMessage(%q) = %s", test.input, err)
			}
			continue
		}

		if test.fail {
			t.Errorf("ParseMessage(%q) should have failed, but did not.", test.input)
			continue
		}

		if msg.Prefix != test.prefix {
			t.Errorf("ParseMessage(%q) got prefix %v, wanted %v", test.input,
				msg.Prefix, test.prefix)
			continue
		}

		if msg.Command != test.command {
			t.Errorf("ParseMessage(%q) got command %v, wanted %v", test.input,
				msg.Command, test.command)
			continue
		}

		if !paramsEqual(msg.Params, test.params) {
			t.Errorf("ParseMessage(%q) got params %q, wanted %q", test.input,
				msg.Params, test.params)
			continue
		}
	}
}

func TestFixLineEnding(t *testing.T) {
	tests := []struct {
		input   string
		output  string
		success bool
	}{
		{"hi", "", false},
		{"hi\n", "hi\r\n", true},
		{"hi\r\n", "hi\r\n", true},
		{"\n", "\r\n", true},
		{"\r\n", "\r\n", true},
	}

	for _, test := range tests {
		out, err := fixLineEnding(test.input)
		if err != nil {
			if !test.success {
				continue
			}

			t.Errorf("fixLineEnding(%s) failed %s, wanted %s", test.input, err,
				test.output)
			continue
		}

		if !test.success {
			t.Errorf("fixLineEnding(%s) succeeded, wanted failure", test.input)
			continue
		}

		if out != test.output {
			t.Errorf("fixLineEnding(%s) = %s, wanted %s", test.input, out,
				test.output)
		}
	}
}

func TestParsePrefix(t *testing.T) {
	var tests = []struct {
		input  string
		prefix string
		index  int
	}{
		{":irc.example.com PRIVMSG", "irc.example.com", 17},
		{":irc.example.com ", "irc.example.com", 17},
		{":irc PRIVMSG ", "irc", 5},
		{"irc.example.com", "", -1},
		{": PRIVMSG ", "", -1},
		{"irc\rexample.com", "", -1},
	}

	for _, test := range tests {
		prefix, index, err := parsePrefix(test.input)

		if err != nil {
			if test.index != -1 {
				t.Errorf("parsePrefix(%q) = error %s", test.input, err.Error())
			}
			continue
		}

		if test.index == -1 {
			t.Errorf("parsePrefix(%q) should have failed, but did not", test.input)
			continue
		}

		if prefix != test.prefix {
			t.Errorf("parsePrefix(%q) = %v, want %v", test.input, prefix,
				test.prefix)
			continue
		}

		if index != test.index {
			t.Errorf("parsePrefix(%q) = %v, want %v", test.input, index,
				test.index)
			continue
		}
	}
}

func TestParseCommand(t *testing.T) {
	var tests = []struct {
		input      string
		command    string
		startIndex int
		newIndex   int
	}{
		{":irc PRIVMSG blah\r\n", "PRIVMSG", 5, 12},
		{":irc 001 :Welcome\r\n", "001", 5, 8},
		{":irc 001\r\n", "001", 5, 8},
		{":irc PRIVMSG ", "PRIVMSG", 5, 12},
		{":irc @01\r\n", "", 5, -1},
		{":irc \r\n", "", 5, -1},
		{":irc  PRIVMSG\r\n", "", 5, -1},
	}

	for _, test := range tests {
		command, newIndex, err := parseCommand(test.input, test.startIndex)

		if err != nil {
			if test.newIndex != -1 {
				t.Errorf("parseCommand(%q) = error %s", test.input, err.Error())
			}
			continue
		}

		if test.newIndex == -1 {
			t.Errorf("parseCommand(%q) should have failed, but did not", test.input)
			continue
		}

		if command != test.command {
			t.Errorf("parseCommand(%q) = %v, want %v", test.input, command,
				test.command)
			continue
		}

		if newIndex != test.newIndex {
			t.Errorf("parseCommand(%q) = %v, want %v", test.input, newIndex,
				test.newIndex)
			continue
		}
	}
}

func TestParseParams(t *testing.T) {
	tests := []struct {
		input    string
		index    int
		params   []string
		newIndex int
	}{
		{":irc 000 hi\r\n", 8, []string{"hi"}, 11},

		// It is valid to have no parameters.
		{":irc 000\r\n", 8, nil, 8},

		// Test last param having no :
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5\r\n", 8, []string{"1", "2",
			"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", "5"}, 38},

		// Test last param having no : nor characters
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 \r\n", 8, []string{"1", "2",
			"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", ""}, 37},

		// Test last param having : but no characters
		{":irc 000 1 2 3 4 5 6 7 8 9 0 1 2 3 4 :\r\n", 8, []string{"1", "2",
			"3", "4", "5", "6", "7", "8", "9", "0", "1", "2", "3", "4", ""}, 38},

		// Malformed because \r should not appear there.
		{":irc 000 \r\r\n", 8, nil, -1},

		// Must not be blank unless last param.
		// While this violates the grammar, I permit it now anyway.
		{":irc 000 \r\n", 8, []string{}, 9},

		{":irc 000 0a 1b\r\n", 8, []string{"0a", "1b"}, 14},

		// If we have a space then there must be a parameter (unless it's the
		// 15th).
		// While this violates the grammar, I permit it now anyway.
		{":irc 000 0 1 \r\n", 8, []string{"0", "1"}, 13},

		// This is a malformed message but the parameter parsing won't catch
		// it. Let overall message parsing get it.
		{":irc 000 a\x00 1 \r\n", 8, []string{"a"}, 10},

		// This is a malformed message.
		// However I allow it. See comment in parseParam() for why.
		{":irc 000 a:bc\r\n", 8, []string{"a:bc"}, 13},
	}

	for _, test := range tests {
		params, newIndex, err := parseParams(test.input, test.index)
		if err != nil {
			if test.newIndex != -1 {
				t.Errorf("parseParams(%q) = %v, want %v", test.input, err, test.params)
			}
			continue
		}

		if test.newIndex == -1 {
			t.Errorf("parseParams(%q) should have failed, but did not", test.input)
			continue
		}

		if !paramsEqual(params, test.params) {
			t.Errorf("parseParams(%q) = %v, wanted %v", test.input, params,
				test.params)
			continue
		}

		if newIndex != test.newIndex {
			t.Errorf("parseParams(%q) index = %v, wanted %v", test.input, newIndex,
				test.newIndex)
			continue
		}
	}
}

func paramsEqual(params1, params2 []string) bool {
	if len(params1) != len(params2) {
		return false
	}

	for i, v := range params1 {
		if params2[i] != v {
			return false
		}
	}

	return true
}

func TestEncodeMessage(t *testing.T) {
	tests := []struct {
		input   Message
		output  string
		success bool
	}{
		{
			Message{
				Command: "PRIVMSG",
				Prefix:  "nick",
				Params:  []string{"nick2", "hi there"},
			},
			":nick PRIVMSG nick2 :hi there\r\n",
			true,
		},
		{
			Message{
				Command: "PRIVMSG",
				Prefix:  "nick",
				Params:  []string{"nick2", " hi there"},
			},
			":nick PRIVMSG nick2 : hi there\r\n",
			true,
		},
		{
			Message{
				Command: "TOPIC",
				Prefix:  "nick",
				Params:  []string{"#test", "hi there"},
			},
			":nick TOPIC #test :hi there\r\n",
			true,
		},

		// We can have zero length TOPIC in TS6 protocol - for when the topic is
		// to be unset.
		{
			Message{
				Command: "TOPIC",
				Prefix:  "nick",
				Params:  []string{"#test", ""},
			},
			":nick TOPIC #test :\r\n",
			true,
		},

		{
			Message{
				Command: "TOPIC",
				Prefix:  "nick",
				Params:  []string{"#test", ":"},
			},
			":nick TOPIC #test ::\r\n",
			true,
		},
	}

	for _, test := range tests {
		buf, err := test.input.Encode()
		if err != nil {
			if test.success {
				t.Errorf("Encode(%s) failed but should succeed: %s", test.input, err)
				continue
			}
			continue
		}

		if !test.success {
			t.Errorf("Encode(%s) succeeded but should fail", test.input)
			continue
		}

		if buf != test.output {
			t.Errorf("Encode(%s) = %s, wanted %s", test.input, buf, test.output)
			continue
		}
	}
}
