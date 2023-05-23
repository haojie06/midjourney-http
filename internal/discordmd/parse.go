package discordmd

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

// calculate hash from prompt and seed
func getHashFromPrompt(prompt, seed string) (hashStr string) {
	// replace all image links with seed, because image links in response will change
	prompt = replaceLinks(prompt, seed)
	// print("get hash from prompt: ", prompt, "\n")
	h := md5.Sum([]byte(prompt))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}

// calculate hash from embeds footer message
func getHashFromEmbeds(message string) (hashStr string) {
	// get seed and replace all links with it
	// remove the command part
	parts := strings.SplitN(message, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	message = parts[1]
	seed, ok := getLastSeedFromMessage(message)
	if !ok {
		return ""
	}
	message = strings.Trim(message, " ")
	message = replaceLinks(message, seed)
	// print("--- get hash from embeds", message, "\n")
	h := md5.Sum([]byte(message))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}

// get hash and prompt from message
func getHashFromMessage(message string) (hashStr, promptStr string) {
	promptRe := regexp.MustCompile(`\*{2}(.+?)\*{2}`)
	matches := promptRe.FindStringSubmatch(message)
	if len(matches) < 2 {
		return "", ""
	}
	promptStr = strings.Trim(matches[1], " ")

	seed, ok := getLastSeedFromMessage(message)
	if !ok {
		return "", ""
	}

	linkRe := regexp.MustCompile(`<https?:\/\/\S+\>`) // link in reply message is different from other links, wrapped with <>
	promptStr = linkRe.ReplaceAllString(promptStr, seed)
	// print("get hash from message: ", promptStr, "\n")
	h := md5.Sum([]byte(promptStr))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}

func getLastSeedFromMessage(message string) (string, bool) {
	seedRe := regexp.MustCompile(`--seed\s+(\d+)`)
	matches := seedRe.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return "", false
	}
	lastMatch := matches[len(matches)-1]
	return lastMatch[1], true
}

// replace links in message with seed
func replaceLinks(message, seed string) string {
	linkRe := regexp.MustCompile(`https?:\/\/\S+`)
	return linkRe.ReplaceAllString(message, seed)
}

// get file uuid from url, eg: https://cdn.discordapp.com/attachments/xxxx/xxxx/account_name.some_prompts_d44f04d2-b81b-49ff-83e6-575d3c02f0f0.png
func getFileIdFromURL(url string) (fileId string) {
	re := regexp.MustCompile(`([a-f\d]{8}(-[a-f\d]{4}){3}-[a-f\d]{12})[^/]*$`)
	match := re.FindAllStringSubmatch(url, -1)
	if len(match) == 0 {
		return
	}
	if len(match[len(match)-1]) < 2 {
		return
	}
	fileId = match[len(match)-1][1]
	return
}

func getImageIndexFromMessage(message string) string {
	re := regexp.MustCompile(`Image #(\d+)`)
	match := re.FindStringSubmatch(message)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}
