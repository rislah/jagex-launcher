package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/pkg/browser"
)

type LauncherApp struct {
	app    fyne.App
	window fyne.Window
	api    *JagexAPI
	config *Config
}

func NewLauncherApp() *LauncherApp {
	a := app.New()
	w := a.NewWindow("RuneScape Launcher")
	w.Resize(fyne.NewSize(600, 400))
	w.SetFixedSize(true)

	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		config = &Config{Accounts: []Account{}}
	}

	launcherApp := &LauncherApp{
		app:    a,
		window: w,
		api:    NewJagexAPI(),
		config: config,
	}

	for i := range config.Accounts {
		config.Accounts[i].characters = launcherApp.fetchAllCharactersFromAccount(&config.Accounts[i])
	}

	return launcherApp
}

func (l *LauncherApp) updateAccount(updated Account) {
	for i, acc := range l.config.Accounts {
		if acc.Username == updated.Username {
			l.config.Accounts[i] = updated
			saveConfig(l.config)
			break
		}
	}
}

func (l *LauncherApp) showMainWindow() {
	accountsList := widget.NewList(
		func() int { return len(l.config.Accounts) },
		func() fyne.CanvasObject {
			usernameLabel := widget.NewLabel("Account Username Placeholder")
			usernameLabel.TextStyle.Bold = true

			characterCountLabel := widget.NewLabel("Characters: 0")
			characterCountLabel.TextStyle.Italic = true

			accountDetails := container.NewVBox(
				usernameLabel,
				characterCountLabel,
			)

			playBtn := widget.NewButtonWithIcon("Play", theme.MediaPlayIcon(), nil)
			removeBtn := widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), nil)

			buttonsContainer := container.NewHBox(
				playBtn,
				removeBtn,
			)

			return container.NewBorder(
				nil,              // Top
				nil,              // Bottom
				nil,              // Leading
				buttonsContainer, // Trailing
				accountDetails,   // Center (will expand to fill available width)
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(l.config.Accounts) {
				return
			}

			account := l.config.Accounts[id]

			borderContainer := obj.(*fyne.Container)

			accountDetails := borderContainer.Objects[0].(*fyne.Container)
			buttonsContainer := borderContainer.Objects[1].(*fyne.Container)

			usernameLabel := accountDetails.Objects[0].(*widget.Label)
			characterCountLabel := accountDetails.Objects[1].(*widget.Label)

			playBtn := buttonsContainer.Objects[0].(*widget.Button)
			removeBtn := buttonsContainer.Objects[1].(*widget.Button)

			usernameLabel.SetText(account.Username)
			characterCountLabel.SetText(fmt.Sprintf("Characters: %d", len(account.characters)))

			playBtn.OnTapped = func() {
				l.launchGame(account)
			}

			removeBtn.OnTapped = func() {
				l.removeAccount(id)
			}

			usernameLabel.Refresh()
			characterCountLabel.Refresh()
			playBtn.Refresh()
			removeBtn.Refresh()
		},
	)

	addAccountBtn := widget.NewButtonWithIcon("Add Account", theme.ContentAddIcon(), func() {
		l.showAddAccountDialog()
	})

	settingsBtn := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		l.showSettingsDialog()
	})

	content := container.NewBorder(
		nil,
		container.New(layout.NewAdaptiveGridLayout(2),
			settingsBtn,
			addAccountBtn,
		),
		nil,
		nil,
		accountsList,
	)

	l.window.SetContent(content)
}

func (l *LauncherApp) showSettingsDialog() {
	pathEntry := widget.NewEntry()
	pathEntry.SetText(l.config.RuneLitePath)

	browseBtn := widget.NewButton("Browse", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err == nil && reader != nil {
				pathEntry.SetText(reader.URI().Path())
			}
		}, l.window)
		fd.Show()
	})

	saveBtn := widget.NewButton("Save", func() {
		l.config.RuneLitePath = pathEntry.Text
		if err := saveConfig(l.config); err != nil {
			dialog.ShowError(err, l.window)
		} else {
			dialog.ShowInformation("Settings Saved", "RuneLite path updated successfully", l.window)
		}
	})

	form := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("RuneLite Path:", pathEntry),
		),
		container.NewHBox(
			browseBtn,
			saveBtn,
		),
	)

	dialog := dialog.NewCustom("Application Settings", "Close", form, l.window)
	dialog.Resize(fyne.NewSize(600, 200))
	dialog.Show()
}

func (l *LauncherApp) showAddAccountDialog() {
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)
	state := generateRandomString(16)

	authURL := l.api.getAuthURL(codeChallenge, state)

	codeEntry := widget.NewEntry()
	codeEntry.SetPlaceHolder("Enter authorization code")

	urlLabel := widget.NewRichTextFromMarkdown(fmt.Sprintf("1. Go to: [%s](%s)", authURL, authURL))
	urlLabel.Wrapping = fyne.TextWrapWord

	step1Content := container.NewVBox(
		widget.NewLabel("Step 1: Authorization"),
		widget.NewLabel("Click below to open your browser for authentication"),
		widget.NewButtonWithIcon("Open browser (Step 1)", theme.MailForwardIcon(), func() {
			browser.OpenURL(authURL)
		}),
		widget.NewLabel("2. Copy the 'code' parameter from the redirect URL:"),
		codeEntry,
	)

	gameTokenEntry := widget.NewEntry()
	gameTokenEntry.SetPlaceHolder("Enter game ID token")

	gameAuthButton := widget.NewButtonWithIcon("Open browser (Step 2)", theme.MailForwardIcon(), func() {})
	gameAuthButton.Disable()

	step2Content := container.NewVBox(
		widget.NewLabel("Step 2: Game Authorization"),
		widget.NewLabel("After entering the code, you'll get another URL."),
		gameAuthButton,
		widget.NewLabel("Copy the 'id_token' parameter from that redirect URL:"),
		gameTokenEntry,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Step 1", step1Content),
		container.NewTabItem("Step 2", step2Content),
	)

	var tokenResp *TokenResponse

	nextBtn := widget.NewButton("Next", func() {
		if tabs.SelectedIndex() == 0 {
			code := strings.TrimSpace(codeEntry.Text)
			if code == "" {
				dialog.ShowError(fmt.Errorf("please enter the authorization code"), l.window)
				return
			}

			var err error
			tokenResp, err = l.api.exchangeCodeForToken(code, codeVerifier)
			if err != nil {
				dialog.ShowError(fmt.Errorf("error exchanging code: %v", err), l.window)
				return
			}

			gameAuthURL := l.api.getGameAuthURL(tokenResp.IDToken, state)
			gameAuthButton.OnTapped = func() {
				browser.OpenURL(gameAuthURL)
			}
			gameAuthButton.Enable()

			tabs.SelectTab(tabs.Items[1])
		} else {
			gameIDToken := strings.TrimSpace(gameTokenEntry.Text)
			if gameIDToken == "" {
				dialog.ShowError(fmt.Errorf("please enter the game ID token"), l.window)
				return
			}
			l.completeAccountSetup(gameIDToken, tokenResp)
		}
	})

	//TODO
	cancelBtn := widget.NewButton("Cancel", func() {})
	buttons := container.NewHBox(cancelBtn, nextBtn)
	content := container.NewBorder(nil, buttons, nil, nil, tabs)

	dialog := dialog.NewCustom("Add Account", "Close", content, l.window)
	dialog.Resize(fyne.NewSize(600, 400))
	dialog.Show()
}

func (l *LauncherApp) completeAccountSetup(gameIDToken string, tokenResp *TokenResponse) {
	userInfo, err := l.api.getUserInfo(tokenResp.AccessToken)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error getting user info: %v", err), l.window)
		return
	}

	sessionResp, err := l.api.createGameSession(gameIDToken)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error creating game session: %v", err), l.window)
		return
	}

	account := Account{
		Username:     userInfo.Nickname,
		RefreshToken: tokenResp.RefreshToken,
		AccessToken:  tokenResp.AccessToken,
		IDToken:      tokenResp.IDToken,
		GameIDToken: gameIDToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix(),
		SessionID:    sessionResp.SessionID,
	}

	l.config.Accounts = append(l.config.Accounts, account)
	saveConfig(l.config)
	l.showMainWindow()
	dialog.ShowInformation("Success", "Account added successfully!", l.window)
}

func (l *LauncherApp) removeAccount(index int) {
	if index >= 0 && index < len(l.config.Accounts) {
		l.config.Accounts = append(l.config.Accounts[:index], l.config.Accounts[index+1:]...)
		saveConfig(l.config)
		l.showMainWindow()
	}
}

func (l *LauncherApp) refreshAuthTokensAndSession(account *Account) error {
	tokenResp, err := l.api.refreshToken(account.RefreshToken)
	if err != nil {
		return err
	}

	account.AccessToken = tokenResp.AccessToken
	account.RefreshToken = tokenResp.RefreshToken
	account.IDToken = tokenResp.IDToken
	account.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()

	sessionResp, err := l.api.createGameSession(account.GameIDToken)
	if err != nil {
		return err
	}
	account.SessionID = sessionResp.SessionID

	l.updateAccount(*account)
	return nil
}

func (l *LauncherApp) fetchAllCharactersFromAccount(account *Account) []Character {
	if account.SessionID == "" {
		return nil
	}

	if time.Now().Unix() >= account.ExpiresAt {
		if err := l.refreshAuthTokensAndSession(account); err != nil {
			dialog.ShowError(fmt.Errorf("error refreshing credentials: %v", err), l.window)
			return nil
		}
	}

	accountsResp, err := l.api.getAccounts(account.SessionID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error getting accounts: %v", err), l.window)
		return nil
	}

	var characters []Character
	for _, a := range *accountsResp {
		characters = append(characters, Character{
			ID:          a.AccountID,
			DisplayName: a.DisplayName,
		})
	}
	return characters
}

func (l *LauncherApp) launchGame(account Account) {
	if time.Now().Unix() >= account.ExpiresAt {
		if err := l.refreshAuthTokensAndSession(&account); err != nil {
			dialog.ShowError(fmt.Errorf("error refreshing credentials: %v", err), l.window)
			return
		}
	}

	account.characters = l.fetchAllCharactersFromAccount(&account)
	l.updateAccount(account)

	if len(account.characters) > 1 {
		l.showCharacterSelection(account)
	} else if len(account.characters) == 1 {
		l.startGame(account.characters[0], account.SessionID)
	} else {
		dialog.ShowError(fmt.Errorf("no characters found for this account"), l.window)
	}
}

func (l *LauncherApp) showCharacterSelection(account Account) {
	var selectedCharacter *Character

	characterList := widget.NewList(
		func() int { return len(account.characters) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Character")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(account.characters) {
				return
			}
			obj.(*widget.Label).SetText(account.characters[id].DisplayName)
		},
	)

	characterList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(account.characters) {
			selectedCharacter = &account.characters[id]
		}
	}

	playBtn := widget.NewButton("Play", func() {
		if selectedCharacter != nil {
			l.startGame(*selectedCharacter, account.SessionID)
		}
	})

	content := container.NewBorder(
		widget.NewLabel("Select Character"),
		playBtn,
		nil,
		nil,
		characterList,
	)

	dialog := dialog.NewCustom("Character Selection", "Cancel", content, l.window)
	dialog.Resize(fyne.NewSize(300, 400))
	dialog.Show()
}

func (l *LauncherApp) startGame(character Character, sessionID string) {
	if l.config.RuneLitePath == "" {
		dialog.ShowError(fmt.Errorf("RuneLite path not configured in settings"), l.window)
		return
	}

	if _, err := os.Stat(l.config.RuneLitePath); os.IsNotExist(err) {
		dialog.ShowError(fmt.Errorf("RuneLite not found at configured path: %s", l.config.RuneLitePath), l.window)
		return
	}

	env := []string{
		"JX_CHARACTER_ID=" + character.ID,
		"JX_DISPLAY_NAME=" + character.DisplayName,
		"JX_SESSION_ID=" + sessionID,
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(l.config.RuneLitePath), ".jar") {
		cmd = exec.Command("java", "-jar", l.config.RuneLitePath)
	} else {
		cmd = exec.Command(l.config.RuneLitePath)
	}

	cmd.Env = append(env, os.Environ()...)

	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("error starting game: %v", err), l.window)
		return
	}

	dialog.ShowInformation("Game Started", fmt.Sprintf("Launching %s", character.DisplayName), l.window)
}

func (l *LauncherApp) Run() {
	l.showMainWindow()
	l.window.ShowAndRun()
}
