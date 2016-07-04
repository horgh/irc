package irc

import (
	"fmt"
)

// Message holds a protocol message.
// See section 2.3.1 in RFC 2812.
type Message struct {
	// Prefix may be blank. It's optional.
	Prefix string

	Command string

	// There are at most 15 parameters.
	Params []string
}

// parseMessage parses a message from the server.
//
// See RFC 2812 Section 2.3.1.
//
// line ends with \n.
func parseMessage(line string) (Message, error) {
	message := Message{}
	index := 0

	// It is optional to have a prefix.
	if line[0] == ':' {
		prefix, prefixIndex, err := parsePrefix(line)
		if err != nil {
			return Message{}, fmt.Errorf("Problem parsing prefix: %s", err.Error())
		}
		index = prefixIndex

		message.Prefix = prefix

		if index >= len(line) {
			return Message{}, fmt.Errorf("Malformed message. Prefix only.")
		}
	}

	// We've either parsed a prefix out or have no prefix.
	command, index, err := parseCommand(line, index)
	if err != nil {
		return Message{}, fmt.Errorf("Problem parsing command: %s", err)
	}

	message.Command = command

	if index >= len(line) {
		return Message{}, fmt.Errorf("Malformed message. Command ends message.")
	}

	// May have params.
	params, index, err := parseParams(line, index)
	if err != nil {
		return Message{}, fmt.Errorf("Problem parsing params: %s", err)
	}

	message.Params = params

	// We should now have CRLF.
	// index should be pointing at the CR after parsing params.
	if index != len(line)-2 || line[index] != '\r' || line[index+1] != '\n' {
		return Message{}, fmt.Errorf("Malformed message. No CRLF found. Looking for end at position %d.", index)
	}

	return message, nil
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
		return "", -1, fmt.Errorf("Line does not start with :")
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
			return "", -1, fmt.Errorf("Invalid character found: [%q]", line[pos])
		}

		pos++
	}

	// We didn't find a space.
	if pos == len(line) {
		return "", -1, fmt.Errorf("No space found")
	}

	// Ensure we have at least one character in the prefix.
	if pos == 1 {
		return "", -1, fmt.Errorf("Prefix is zero length")
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
			return "", -1, fmt.Errorf("Unexpected character after command: [%q]",
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
	return line[index:newIndex], newIndex, nil
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
				return nil, -1, fmt.Errorf("Problem parsing parameter: %s", err)
			}
			newIndex = paramIndex

			params = append(params, param)

			continue
		}

		param, newIndex, err := parseParamLast(line, newIndex)
		if err != nil {
			return nil, -1, fmt.Errorf("Problem parsing last parameter: %s", err)
		}

		params = append(params, param)

		return params, newIndex, nil
	}

	return nil, -1, fmt.Errorf("Malformed params. Not terminated properly.")
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
		return "", -1, fmt.Errorf("Malformed param. No leading space.")
	}

	newIndex++

	if len(line) == newIndex {
		return "", -1, fmt.Errorf("Malformed parameter. End of string after space.")
	}

	// SPACE ":" trailing
	if line[newIndex] == ':' {
		newIndex++

		if len(line) == newIndex {
			return "", -1, fmt.Errorf("Malformed parameter. End of string after :.")
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

	paramIndexStart := newIndex

	for newIndex < len(line) {
		// XXX: We should not permit ':' either. However in practice it appears
		//   IRC servers in the wild will send middle parameters with : inside.
		//   e.g., ircd-ratbox in its 005 command.
		if line[newIndex] == '\x00' || line[newIndex] == '\r' ||
			line[newIndex] == '\n' || line[newIndex] == ' ' {
			break
		}
		newIndex++
	}

	// Must have at least one character in this case.
	if paramIndexStart == newIndex {
		return "", -1, fmt.Errorf("Malformed message. Param with zero characters.")
	}

	return line[paramIndexStart:newIndex], newIndex, nil
}

// parseParamLast takes the final case when parsing params.
//
// Specifically, the [ SPACE [ ":" ] trailing ] case.
func parseParamLast(line string, index int) (string, int, error) {
	newIndex := index

	if line[newIndex] != ' ' {
		return "", -1, fmt.Errorf("Malformed param. No leading space.")
	}

	newIndex++

	if newIndex == len(line) {
		return "", -1, fmt.Errorf("Malformed param. Space ends message.")
	}

	// It is valid for there to be no :
	if line[newIndex] == ':' {
		newIndex++

		if newIndex == len(line) {
			return "", -1, fmt.Errorf("Malformed param. : ends message.")
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
