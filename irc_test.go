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
		{":irc \r\n", "", "", []string{}, true},
		{":irc PRIVMSG \r\n", "", "", []string{}, true},
		{"PRIVMSG\r\n", "", "PRIVMSG", []string{}, false},
		{"PRIVMSG :hi there\r\n", "", "PRIVMSG", []string{"hi there"}, false},
		{": PRIVMSG \r\n", "", "", []string{}, true},
		{"ir\rc\r\n", "", "", []string{}, true},

		{":irc PRIVMSG blah\r\n", "irc", "PRIVMSG", []string{"blah"}, false},
		{":irc 001 :Welcome\r\n", "irc", "001", []string{"Welcome"}, false},
		{":irc 001\r\n", "irc", "001", []string{}, false},
		{":irc PRIVMSG \r\n", "", "", []string{}, true},
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
		{":irc 000 \r\n", "", "", []string{}, true},

		{":irc 000 0a 1b\r\n", "irc", "000", []string{"0a", "1b"}, false},

		// If we have a space then there must be a parameter (unless it's the
		// 15th).
		{":irc 000 0 1 \r\n", "", "", []string{}, true},

		{":irc 000 a\x00 1 \r\n", "", "", []string{}, true},

		// Malformed because : inside a middle
		// However I permit this. See comment in parseParams().
		{":irc 000 a:bc\r\n", "irc", "000", []string{"a:bc"}, false},

		{":irc 000 hi :there yes\r\n", "irc", "000", []string{"hi", "there yes"},
			false},

		// Fail because inside :
		{":irc 000 hi:hi :no no", "", "", []string{}, true},

		// Fails but should not. Trailing whitespace.
		{":irc MODE #test +o user  ", "irc", "MODE", []string{"+o", "user"}, false},
	}

	for _, test := range tests {
		msg, err := parseMessage(test.input)
		if err != nil {
			if test.fail != true {
				t.Errorf("parseMessage(%q) = %s", test.input, err)
			}
			continue
		}

		if msg.Prefix != test.prefix {
			t.Errorf("parseMessage(%q) got prefix %v, wanted %v", test.input,
				msg.Prefix, test.prefix)
			continue
		}

		if msg.Command != test.command {
			t.Errorf("parseMessage(%q) got command %v, wanted %v", test.input,
				msg.Command, test.command)
			continue
		}

		if !paramsEqual(msg.Params, test.params) {
			t.Errorf("parseMessage(%q) got params %q, wanted %q", test.input,
				msg.Params, test.params)
			continue
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
		{":irc 000 \r\n", 8, nil, -1},

		{":irc 000 0a 1b\r\n", 8, []string{"0a", "1b"}, 14},

		// If we have a space then there must be a parameter (unless it's the
		// 15th).
		{":irc 000 0 1 \r\n", 8, nil, -1},

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
