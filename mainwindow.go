package main

import (
	"encoding/json"
	"fmt"
	"github.com/chenqinghe/redis-desktop/i18n"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type MainWindowEX struct {
	*walk.MainWindow

	logFile string

	lang i18n.Lang

	LE_host     *walk.LineEdit
	LE_port     *walk.LineEdit
	LE_password *walk.LineEdit

	LE_command *walk.LineEdit

	PB_connect *PushButtonEx

	sessionFile string
	LB_sessions *ListBoxEX

	TW_screenGroup *TabWidgetEx
}

func (mw *MainWindowEX) saveSessions(sessions []session) error {
	data, err := json.Marshal(sessions)
	if err != nil {
		return err
	}
RETRY:
	if err := ioutil.WriteFile(mw.sessionFile, data, os.ModePerm); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(mw.sessionFile), os.ModePerm); err != nil {
				return err
			}
			goto RETRY
		}
		return err
	}
	return nil
}

func (mw *MainWindowEX) SetSessionFile(file string) {
	mw.sessionFile = file
}

func (mw *MainWindowEX) LoadSession() error {
	data, err := ioutil.ReadFile(mw.sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sessions := make([]session, 0)
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	mw.LB_sessions.AddSessions(sessions)
	return nil
}

func (mw *MainWindowEX) importSession(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	sessions := make([]session, 0)
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}
	mw.LB_sessions.AddSessions(sessions)
	return nil
}

func createMainWindow(lang i18n.Lang) *MainWindowEX {
	mw := &MainWindowEX{
		lang:           lang,
		PB_connect:     new(PushButtonEx),
		LB_sessions:    new(ListBoxEX),
		TW_screenGroup: new(TabWidgetEx),
	}
	mw.PB_connect.root = mw
	mw.LB_sessions.root = mw
	mw.TW_screenGroup.root = mw
	err := MainWindow{
		Title:    mw.lang.Tr("mainwindow.title"),
		MinSize:  Size{600, 400},
		AssignTo: &mw.MainWindow,
		Layout:   VBox{MarginsZero: true},
		//Background: SolidColorBrush{Color: walk.RGB(132, 34, 234)},
		MenuItems: []MenuItem{
			Menu{
				Text: mw.lang.Tr("mainwindow.menu.file"),
				Items: []MenuItem{
					Action{
						Text: mw.lang.Tr("mainwindow.menu.file.import"),
						OnTriggered: func() {
							dlg := &walk.FileDialog{
								Title: "choose a file", // string
							}
							accepted, err := dlg.ShowOpen(mw)
							if err != nil {
								walk.MsgBox(mw, "ERROR", "Open FileDialog:"+err.Error(), walk.MsgBoxIconError)
								return
							}
							if accepted {
								if err := mw.importSession(dlg.FilePath); err != nil {
									walk.MsgBox(mw, "ERROR", "Import Session:"+err.Error(), walk.MsgBoxIconError)
									return
								}
							}
						},
					},
					Action{
						Text: mw.lang.Tr("mainwindow.menu.file.export"),
						OnTriggered: func() {
							dlg := &walk.FileDialog{
								Title: "save to file",
							}
							accepted, err := dlg.ShowSave(mw)
							if err != nil {
								walk.MsgBox(mw, "ERROR", "Open FileDialog:"+err.Error(), walk.MsgBoxIconError)
								return
							}
							if accepted {
								sessions := mw.LB_sessions.GetSessions()
								data, err := json.Marshal(sessions)
								if err != nil {
									walk.MsgBox(mw, "ERROR", "Save Session Error:"+err.Error(), walk.MsgBoxIconError)
									return
								}
								if err := ioutil.WriteFile(dlg.FilePath, data, os.ModePerm); err != nil {
									walk.MsgBox(mw, "ERROR", "Write Session Error:"+err.Error(), walk.MsgBoxIconError)
									return
								}
							}
						},
					},
				},
			},
			Menu{
				Text: mw.lang.Tr("mainwindow.menu.edit"),
				Items: []MenuItem{
					Action{
						Text: mw.lang.Tr("mainwindow.menu.edit.clear"),
						OnTriggered: func() {
							mw.TW_screenGroup.CurrentPage().content.ClearScreen()
						},
					},
				},
			},
			Menu{
				Text: mw.lang.Tr("mainwindow.menu.setting"),
				Items: []MenuItem{
					Action{
						Text:        mw.lang.Tr("mainwindow.menu.setting.theme"),
						OnTriggered: nil,
					},
					Action{
						Text:        mw.lang.Tr("mainwindow.menu.logpath"),
						OnTriggered: nil,
					},
				},
			},
			Menu{
				Text: mw.lang.Tr("mainwindow.menu.run"),
				Items: []MenuItem{
					Action{
						Text: mw.lang.Tr("mainwindow.menu.run.batch"),
						OnTriggered: func() {
							curTabpage := mw.TW_screenGroup.CurrentPage()
							if curTabpage == nil {
								walk.MsgBox(mw, "INFO", "当前没有打开的会话", walk.MsgBoxIconInformation)
								return
							}
							batchRun(mw)
						},
					},
				},
			},
			Menu{
				Text: mw.lang.Tr("mainwindow.menu.help"),
				Items: []MenuItem{
					Action{
						Text: mw.lang.Tr("mainwindow.menu.help.source"),
						OnTriggered: func() {
							startPage("https://github.com/chenqinghe/redis-desktop")
						},
					},
					Action{
						Text:        mw.lang.Tr("mainwindow.menu.help.bug"),
						OnTriggered: startIssuePage,
					},
				},
			},
		},
		Children: []Widget{
			LineEdit{
				AssignTo: &mw.LE_command,
				Visible:  false,
			},
			VSplitter{
				Children: []Widget{
					Composite{
						MaxSize: Size{0, 50},
						Layout:  HBox{},
						Children: []Widget{
							Label{Text: mw.lang.Tr("mainwindow.labelhost")},
							LineEdit{AssignTo: &mw.LE_host},
							Label{Text: mw.lang.Tr("mainwindow.labelport")},
							LineEdit{AssignTo: &mw.LE_port},
							Label{Text: mw.lang.Tr("mainwindow.labelpassword")},
							LineEdit{AssignTo: &mw.LE_password, PasswordMode: true},
							PushButton{
								Text:      mw.lang.Tr("mainwindow.PBconnect"),
								AssignTo:  &mw.PB_connect.PushButton,
								OnClicked: mw.PB_connect.OnClick,
							},
						},
					},
					Composite{
						Layout: HBox{MarginsZero: true},
						Children: []Widget{
							ListBox{
								MaxSize:  Size{200, 0},
								AssignTo: &mw.LB_sessions.ListBox,
								Model:    mw.LB_sessions.Model,
								Font: Font{
									Family:    "Consolas",
									PointSize: 10,
								},
								OnItemActivated: func() {
									if mw.LB_sessions.CurrentIndex() >= 0 {
										mw.TW_screenGroup.startNewSession(mw.LB_sessions.CurrentSession())
									}
								},
								OnSelectedIndexesChanged: func() { mw.LB_sessions.EnsureItemVisible(0) },
								OnCurrentIndexChanged:    func() {},
								MultiSelection:           false,
								ContextMenuItems: []MenuItem{
									Action{
										Text:        mw.lang.Tr("mainwindow.LBsessions.menu.deletesession"),
										OnTriggered: mw.LB_sessions.RemoveSelectedSession,
									},
								},
							},
							TabWidget{
								AssignTo: &mw.TW_screenGroup.TabWidget,
								Pages: []TabPage{
									TabPage{
										Title: "home",
										Image: "img/home.ico",
										Content: ImageView{
											Mode:  ImageViewModeStretch,
											Image: "img/cover.png",
										},
									},
								},
								ContentMarginsZero: true,
							},
						},
					},
				},
			},
		},
	}.Create()
	if err != nil {
		log.Fatalln(err)
	}

	icon, _ := walk.NewIconFromFile("img/redis.ico")
	mw.SetIcon(icon)

	return mw
}

func startIssuePage() {
	body := url.QueryEscape(fmt.Sprintf(issueTemplate, VERSION))
	uri := fmt.Sprintf("https://github.com/chenqinghe/redis-desktop/issues/new?body=%s", body)
	startPage(uri)
}

func startPage(uri string) {
	cmd := exec.Command("cmd", "/C", "start", uri)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorln("exec cmd error:", err)
	}
}

func batchRun(p *MainWindowEX) {
	var dlg *walk.Dialog
	var cmdContent *walk.TextEdit

	if _, err := (Dialog{
		Title:    "批量运行命令",
		AssignTo: &dlg,
		MinSize:  Size{500, 500},
		Layout: VBox{Margins: Margins{
			Left:   10, //int
			Top:    10, //int
			Right:  10, //int
			Bottom: 10, //int
		}},
		Children: []Widget{
			Label{Text: "请在下面输入要执行的命令，每行一条..."},
			TextEdit{
				AssignTo: &cmdContent,
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						Text: "确定",
						OnClicked: func() {
							content := cmdContent.Text()
							dlg.Close(0)
							cmds := strings.Split(content, "\r\n")
							curTabpage := p.TW_screenGroup.CurrentPage()
							if curTabpage == nil {
								walk.MsgBox(p, "INFO", "当前没有打开的会话", walk.MsgBoxIconInformation)
								return
							}
							for _, v := range cmds {
								v = strings.TrimSpace(v)
								if len(v) > 0 {
									curTabpage.content.AppendText(v)
									curTabpage.content.runCmd(v)
								}
							}
						},
					},
					PushButton{
						Text: "取消",
						OnClicked: func() {
							dlg.Close(0)
						},
					},
				},
			},
		},
	}).Run(p); err != nil {
		logrus.Errorln("show batch run dialog error:", err)
	}
}
