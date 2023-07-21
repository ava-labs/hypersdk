package cli

import (
	"strings"

	"github.com/ava-labs/hypersdk/crypto"
	"github.com/manifoldco/promptui"
)

type Handler struct {
	c Controller
}

func New(c Controller) *Handler {
	return &Handler{c}
}

func (h *Handler) PromptAddress(label string) (crypto.PublicKey, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			_, err := h.c.ParseAddress(input)
			return err
		},
	}
	recipient, err := promptText.Run()
	if err != nil {
		return crypto.EmptyPublicKey, err
	}
	recipient = strings.TrimSpace(recipient)
	return h.c.ParseAddress(recipient)
}
