package gui

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"unicode"

	"github.com/gage-technologies/mistral-go"
	"github.com/jesseduffield/gocui"
)

func (gui *Gui) handleEditorKeypress(textArea *gocui.TextArea, key gocui.Key, ch rune, mod gocui.Modifier, allowMultiline bool) bool {
	switch {
	case key == gocui.KeyHome:
		textArea.TypeString(gui.generateCommitName())
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		textArea.BackSpaceChar()
	case key == gocui.KeyCtrlD || key == gocui.KeyDelete:
		textArea.DeleteChar()
	case key == gocui.KeyArrowDown:
		textArea.MoveCursorDown()
	case key == gocui.KeyArrowUp:
		textArea.MoveCursorUp()
	case (key == gocui.KeyArrowLeft || ch == 'b') && (mod&gocui.ModAlt) != 0:
		textArea.MoveLeftWord()
	case key == gocui.KeyArrowLeft || key == gocui.KeyCtrlB:
		textArea.MoveCursorLeft()
	case (key == gocui.KeyArrowRight || ch == 'f') && (mod&gocui.ModAlt) != 0:
		textArea.MoveRightWord()
	case key == gocui.KeyArrowRight || key == gocui.KeyCtrlF:
		textArea.MoveCursorRight()
	case key == gocui.KeyEnter:
		if allowMultiline {
			textArea.TypeRune('\n')
		} else {
			return false
		}
	case key == gocui.KeySpace:
		textArea.TypeRune(' ')
	case key == gocui.KeyInsert:
		textArea.ToggleOverwrite()
	case key == gocui.KeyCtrlU:
		textArea.DeleteToStartOfLine()
	case key == gocui.KeyCtrlK:
		textArea.DeleteToEndOfLine()
	case key == gocui.KeyCtrlA || key == gocui.KeyHome:
		textArea.GoToStartOfLine()
	case key == gocui.KeyCtrlE || key == gocui.KeyEnd:
		textArea.GoToEndOfLine()
	case key == gocui.KeyCtrlW:
		textArea.BackSpaceWord()
	case key == gocui.KeyCtrlY:
		textArea.Yank()

	case unicode.IsPrint(ch):
		textArea.TypeRune(ch)
	default:
		return false
	}

	return true
}

func (gui *Gui) generateCommitName() string {
	cmd := exec.Command("git", "diff", "--cached") // staged uniquement
	output, err := cmd.Output()
	if err != nil {
		return "fix: failed to get git diff"
	}

	client := mistral.NewMistralClientDefault(os.Getenv("MISTRAL_API_KEY"))

	chatRes, err := client.Chat("mistral-tiny", []mistral.ChatMessage{
		{
			Role: mistral.RoleUser,
			Content: `You are an assistant that generates Git commit messages. 
			Please respond in strict JSON format like this:
			{ "titre": "type: short title"}

			The message must follow Git commit conventions like feat:, fix:, docs:, and stay under 60 characters for the title.

			Here is the staged diff:
			` + "```diff\n" + string(output) + "\n```",
		},
	}, &mistral.ChatRequestParams{
		ResponseFormat: mistral.ResponseFormatJsonObject,
		MaxTokens:      100,
		TopP:           1,
		Temperature:    1,
	})

	if err != nil {
		log.Printf("Error getting chat completion: %v", err)
		return err.Error()
	}

	if len(chatRes.Choices) == 0 {
		return "fix: no response from Mistral"
	}

	response := chatRes.Choices[0].Message.Content

	var result struct {
		Titre string `json:"titre"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return "fix: invalid JSON format"
	}

	return result.Titre
}

// we've just copy+pasted the editor from gocui to here so that we can also re-
// render the commit message length on each keypress
func (gui *Gui) commitMessageEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {

	v.SetCursor(0, 0) // Assure-toi que le curseur est au début
	matched := gui.handleEditorKeypress(v.TextArea, key, ch, mod, false)

	// Mise à jour de l'affichage de l'éditeur
	v.RenderTextArea()

	// Mise à jour du sous-titre si nécessaire
	gui.c.Contexts().CommitMessage.RenderSubtitle()

	return matched
}

func (gui *Gui) commitDescriptionEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	matched := gui.handleEditorKeypress(v.TextArea, key, ch, mod, true)
	v.RenderTextArea()
	return matched
}

func (gui *Gui) promptEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	matched := gui.handleEditorKeypress(v.TextArea, key, ch, mod, false)

	v.RenderTextArea()

	suggestionsContext := gui.State.Contexts.Suggestions
	if suggestionsContext.State.FindSuggestions != nil {
		input := v.TextArea.GetContent()
		suggestionsContext.State.AsyncHandler.Do(func() func() {
			suggestions := suggestionsContext.State.FindSuggestions(input)
			return func() { suggestionsContext.SetSuggestions(suggestions) }
		})
	}

	return matched
}

func (gui *Gui) searchEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	matched := gui.handleEditorKeypress(v.TextArea, key, ch, mod, false)
	v.RenderTextArea()

	searchString := v.TextArea.GetContent()

	gui.helpers.Search.OnPromptContentChanged(searchString)

	return matched
}
