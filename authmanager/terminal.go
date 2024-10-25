package authmanager

import (
	"bufio"
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
	"os"
	"strings"
	"syscall"
	"tdlib/config"
)

// TerminalPrompt implements auth.UserAuthenticator prompting the terminal for
// input.
//
// This is only example implementation, you should not use it in your code.
// Copy it and modify to fit your needs.
type TerminalPrompt struct {
	AppConfig   *config.AppConfig
	PhoneNumber string
}

func NewTerminalPrompt(appConfig config.AppConfig) *TerminalPrompt {
	return &TerminalPrompt{
		PhoneNumber: appConfig.PhoneNumber,
	}
}

func (TerminalPrompt) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("signing up not implemented in TerminalPrompt")
}

func (TerminalPrompt) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (TerminalPrompt) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter code: ")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (a TerminalPrompt) Phone(_ context.Context) (string, error) {
	if a.PhoneNumber != "" {
		return a.PhoneNumber, nil
	}
	fmt.Print("Enter phone in international format (e.g. +1234567890): ")
	phone, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(phone), nil
}

func (TerminalPrompt) Password(_ context.Context) (string, error) {
	fmt.Print("Enter 2FA password: ")
	bytePwd, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytePwd)), nil
}
