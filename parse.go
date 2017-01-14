// Package irc implements parsing and encoding of IRC protocol messages.
package irc

import (
	"errors"
	"fmt"
	"strings"
)

// MaxLineLength is the maximum protocol message line length.
// From RFC 2812 section 2.3. It includes CRLF.
const MaxLineLength = 512

// ErrTruncated is the error returned by Encode if the message gets truncated
// due to encoding to more than MaxLineLength bytes.
var ErrTruncated = errors.New("message truncated")

// Message holds a protocol message.
// See section 2.3.1 in RFC 2812.
type Message struct {
	// Prefix may be blank. It's optional.
	Prefix string

	Command string

	// There are at most 15 parameters.
	Params []string
}

func (m Message) String() string {
	return fmt.Sprintf("Prefix [%s] Command [%s] Params%q", m.Prefix, m.Command,
		m.Params)
}

// Encode turns the in memory representation of the memory into the IRC
// protocol message string.
//
// It does not enforce command specific semantics. It is instead responsible
// only for placing prefix, command, and parameters.
func (m Message) Encode() (string, error) {
	s := ""

	if len(m.Prefix) > 0 {
		s += ":" + m.Prefix + " "
	}

	s += m.Command

	if len(s)+2 > MaxLineLength {
		return "", fmt.Errorf("message with only prefix/command is too long")
	}

	truncated := false

	for i, param := range m.Params {
		// If we have a ":" or " ", then we need to prefix the parameter with ":".
		// This happening is only valid for the last ("trailing") parameter.
		//
		// Also, the trailing parameter may be an empty string. Ensure it shows up
		// by adding a :. This can happen e.g. from ircd-ratbox in TOPIC unset
		// command (server protocol). Modern IRC and ircv3 both specify this as
		// valid (that the last parameter may be empty). Also, RFC 2812's grammar
		// permits it. The ":" is optional only if this is the 15th parameter. But
		// it is valid.
		if idx := strings.IndexAny(param, ": "); idx != -1 || len(param) == 0 {
			param = ":" + param

			// This must be the last parameter.
			if i+1 != len(m.Params) {
				return "", fmt.Errorf("parameter problem: ':' or ' ' outside last parameter")
			}
		}

		// If we add the parameter as is, do we exceed the maximum length?
		if len(s)+1+len(param)+2 > MaxLineLength {
			// Two cases: Either we can truncate the parameter and include a portion
			// of it, or the parameter is too short to include at all. If it is too
			// short to include, then don't add the space separator either. In the
			// cases where this is not the 15th parameter having a trailing space is
			// not valid, but even in the case of the 15th parameter, we don't need
			// it.

			// Claim the space separator (1) and ending (2) as used. Then we can tell
			// how many bytes are available for the parameter as it is.
			lengthUsed := len(s) + 1 + 2
			lengthAvailable := MaxLineLength - lengthUsed

			// Note: If we prefixed the parameter with : then it's possible we include
			// only the : here (if length available is 1). This is perhaps a little
			// odd but I don't think problematic.

			if lengthAvailable > 0 {
				s += " " + param[0:lengthAvailable]
			}

			truncated = true

			break
		}

		s += " " + param
	}

	s += "\r\n"

	if truncated {
		return s, ErrTruncated
	}

	return s, nil
}

// ParseMessage parses a protocol message from the client/server.
//
// See RFC 2812 Section 2.3.1.
//
// line ends with \n.
func ParseMessage(line string) (Message, error) {
	line, err := fixLineEnding(line)
	if err != nil {
		return Message{}, fmt.Errorf("line does not have a valid ending: %s", line)
	}

	truncated := false

	if len(line) > MaxLineLength {
		truncated = true

		line = line[0:MaxLineLength-2] + "\r\n"
	}

	message := Message{}
	index := 0

	// It is optional to have a prefix.
	if line[0] == ':' {
		prefix, prefixIndex, err := parsePrefix(line)
		if err != nil {
			return Message{}, fmt.Errorf("problem parsing prefix: %s", err)
		}
		index = prefixIndex

		message.Prefix = prefix

		if index >= len(line) {
			return Message{}, fmt.Errorf("malformed message. Prefix only")
		}
	}

	// We've either parsed a prefix out or have no prefix.
	command, index, err := parseCommand(line, index)
	if err != nil {
		return Message{}, fmt.Errorf("problem parsing command: %s", err)
	}

	message.Command = command

	// May have params.
	params, index, err := parseParams(line, index)
	if err != nil {
		return Message{}, fmt.Errorf("problem parsing params: %s", err)
	}

	message.Params = params

	// We should now have CRLF.
	// index should be pointing at the CR after parsing params.
	if index != len(line)-2 || line[index] != '\r' || line[index+1] != '\n' {
		return Message{}, fmt.Errorf("malformed message. No CRLF found. Looking for end at position %d", index)
	}

	if truncated {
		return message, ErrTruncated
	}

	return message, nil
}

// fixLineEnding tries to ensure the line ends with CRLF.
//
// If it ends with only LF, add a CR.
func fixLineEnding(line string) (string, error) {
	if len(line) == 0 {
		return "", fmt.Errorf("line is blank")
	}

	if len(line) == 1 {
		if line[0] == '\n' {
			return "\r\n", nil
		}

		return "", fmt.Errorf("line does not end with LF")
	}

	lastIndex := len(line) - 1
	secondLastIndex := lastIndex - 1

	if line[secondLastIndex] == '\r' && line[lastIndex] == '\n' {
		return line, nil
	}

	if line[lastIndex] == '\n' {
		return line[:lastIndex] + "\r\n", nil
	}

	return "", fmt.Errorf("line has no ending CRLF or LF")
}

// parsePrefix parses out the prefix portion of a string.
//
// line begins with : and ends with \n.
//
// If there is no error we return the prefix and the position after
// the SPACE.
// This means the index points to the first character of the command (in a well
// formed message). We do not confirm there actually is a character.
//
// We are parsing this:
// message    =  [ ":" prefix SPACE ] command [ params ] crlf
// prefix     =  servername / ( nickname [ [ "!" user ] "@" host ] )
//
// TODO: Enforce length limits
// TODO: Enforce character / format more strictly.
//   Right now I don't do much other than ensure there is no space.
func parsePrefix(line string) (string, int, error) {
	pos := 0

	if line[pos] != ':' {
		return "", -1, fmt.Errorf("line does not start with ':'")
	}

	for pos < len(line) {
		// Prefix ends with a space.
		if line[pos] == ' ' {
			break
		}

		// Basic character check.
		// I'm being very lenient here right now. Servername and hosts should only
		// allow [a-zA-Z0-9]. Nickname can have any except NUL, CR, LF, " ". I
		// choose to accept anything nicks can.
		if line[pos] == '\x00' || line[pos] == '\n' || line[pos] == '\r' {
			return "", -1, fmt.Errorf("invalid character found: %q", line[pos])
		}

		pos++
	}

	// We didn't find a space.
	if pos == len(line) {
		return "", -1, fmt.Errorf("no space found")
	}

	// Ensure we have at least one character in the prefix.
	if pos == 1 {
		return "", -1, fmt.Errorf("prefix is zero length")
	}

	// Return the prefix without the space.
	// New index is after the space.
	return line[1:pos], pos + 1, nil
}

// parseCommand parses the command portion of a message from the server.
//
// We start parsing at the given index in the string.
//
// We return the command portion and the index just after the command.
//
// ABNF:
// message    =  [ ":" prefix SPACE ] command [ params ] crlf
// command    =  1*letter / 3digit
// params     =  *14( SPACE middle ) [ SPACE ":" trailing ]
//            =/ 14( SPACE middle ) [ SPACE [ ":" ] trailing ]
func parseCommand(line string, index int) (string, int, error) {
	newIndex := index

	// Parse until we hit a non-letter or non-digit.
	for newIndex < len(line) {
		// Digit
		if line[newIndex] >= 48 && line[newIndex] <= 57 {
			newIndex++
			continue
		}

		// Letter
		if line[newIndex] >= 65 && line[newIndex] <= 122 {
			newIndex++
			continue
		}

		// Must be a space or CR.
		if line[newIndex] != ' ' &&
			line[newIndex] != '\r' {
			return "", -1, fmt.Errorf("unexpected character after command: %q",
				line[newIndex])
		}
		break
	}

	// 0 length command is not valid.
	if newIndex == index {
		return "", -1, fmt.Errorf("0 length command found")
	}

	// TODO: Enforce that we either have 3 digits or all letters.

	// Return command string without space or CR.
	// New index is at the CR or space.
	return strings.ToUpper(line[index:newIndex]), newIndex, nil
}

// parseParams parses the params part of a message.
//
// The given index points to the first character in the params.
// There may not actually be any params. In that case we don't raise an
// error but simply do not return any.
//
// We return each param (stripped of : in the case of 'trailing') and the
// index after the params end.
//
// Note there may be blank parameters in some cases. Specifically since
// trailing accepts 0 length as valid.
//
// Relevant parts of ABNF from RFC 2812 section 2.3.1:
// message    =  [ ":" prefix SPACE ] command [ params ] crlf
// params     =  *14( SPACE middle ) [ SPACE ":" trailing ]
//            =/ 14( SPACE middle ) [ SPACE [ ":" ] trailing ]
// middle     =  nospcrlfcl *( ":" / nospcrlfcl )
// trailing   =  *( ":" / " " / nospcrlfcl )
// nospcrlfcl =  %x01-09 / %x0B-0C / %x0E-1F / %x21-39 / %x3B-FF
//               ; any octet except NUL, CR, LF, " " and ":"
func parseParams(line string, index int) ([]string, int, error) {
	newIndex := index
	var params []string

	for newIndex < len(line) {
		if line[newIndex] != ' ' {
			return params, newIndex, nil
		}

		if len(params) < 14 {
			param, paramIndex, err := parseParam(line, newIndex)
			if err != nil {
				// At this point we should always have at least one character in the
				// param. However it is common in the wild (ratbox, quassel) for there
				// to be a single space character before CRLF. Permit it here.
				// Set index to point after the spurious " " as though we consumed it.
				if err.Error() == "param with zero characters" &&
					newIndex+3 == len(line) && line[newIndex] == ' ' &&
					line[newIndex+1] == '\r' && line[newIndex+2] == '\n' {
					return params, newIndex + 1, nil
				}
				return nil, -1, fmt.Errorf("problem parsing parameter: %s", err)
			}
			newIndex = paramIndex

			params = append(params, param)

			continue
		}

		param, newIndex, err := parseParamLast(line, newIndex)
		if err != nil {
			return nil, -1, fmt.Errorf("problem parsing last parameter: %s", err)
		}

		params = append(params, param)

		return params, newIndex, nil
	}

	return nil, -1, fmt.Errorf("malformed params. Not terminated properly")
}

// parseParam parses out a single parameter term.
//
// index points to a space.
//
// We return the parameter (stripped of : in the case of trailing) and the
// index after the parameter ends.
func parseParam(line string, index int) (string, int, error) {
	newIndex := index

	if line[newIndex] != ' ' {
		return "", -1, fmt.Errorf("malformed param. No leading space")
	}

	newIndex++

	if len(line) == newIndex {
		return "", -1, fmt.Errorf("malformed parameter. End of string after space")
	}

	// SPACE ":" trailing
	if line[newIndex] == ':' {
		newIndex++

		if len(line) == newIndex {
			return "", -1, fmt.Errorf("malformed parameter. End of string after ':'")
		}

		// It is valid for there to be no characters.
		// Because: trailing   =  *( ":" / " " / nospcrlfcl )

		paramIndexStart := newIndex

		for newIndex < len(line) {
			if line[newIndex] == '\x00' || line[newIndex] == '\r' ||
				line[newIndex] == '\n' {
				break
			}
			newIndex++
		}

		return line[paramIndexStart:newIndex], newIndex, nil
	}

	// SPACE middle
	// middle     =  nospcrlfcl *( ":" / nospcrlfcl )
	// nospcrlfcl = any octet except NUL, CR, LF, " ", ":"
	// This means the first character must not be any of those, but afterwards :
	// may appear.
	//
	// We know from the above check (for SPACE ":" trailing) that it is NOT ":",
	// so we can take all except NUL, CR, LF, " ".

	// paramIndexStart points at the character after the space.
	paramIndexStart := newIndex

	for newIndex < len(line) {
		if line[newIndex] == '\x00' || line[newIndex] == '\r' ||
			line[newIndex] == '\n' || line[newIndex] == ' ' {
			break
		}
		newIndex++
	}

	// Must have at least one character in this case. See grammar for 'middle'.
	if paramIndexStart == newIndex {
		return "", -1, fmt.Errorf("param with zero characters")
	}

	return line[paramIndexStart:newIndex], newIndex, nil
}

// parseParamLast takes the final case when parsing params.
//
// Specifically, the [ SPACE [ ":" ] trailing ] case.
func parseParamLast(line string, index int) (string, int, error) {
	newIndex := index

	if line[newIndex] != ' ' {
		return "", -1, fmt.Errorf("malformed param. No leading space")
	}

	newIndex++

	// If we're at the end of the string, then something is wrong. While the
	// parameter may be blank, there should at least be CRLF remaining.
	// It's valid for there to be no characters.
	if newIndex == len(line) {
		return "", -1, fmt.Errorf("malformed param. Space ends message")
	}

	// It is valid for there to be no :
	if line[newIndex] == ':' {
		newIndex++

		// See above. We should have at least CRLF.
		if newIndex == len(line) {
			return "", -1, fmt.Errorf("malformed param. : ends message")
		}
	}

	// It is valid for there to be no characters.

	paramStartIndex := newIndex

	for newIndex < len(line) {
		if line[newIndex] == '\x00' || line[newIndex] == '\r' ||
			line[newIndex] == '\n' {
			break
		}
		newIndex++
	}

	return line[paramStartIndex:newIndex], newIndex, nil
}
