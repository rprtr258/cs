package main

import (
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type randomItemGenerator struct{}

func (randomItemGenerator) next() item { return item{} }

type delegateKeyMap struct {
	remove key.Binding
}

func newDelegateKeyMap() *delegateKeyMap { return &delegateKeyMap{} }

type delegate struct{}

func (delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {}
func (delegate) Height() int                                                 { return 1 }
func (delegate) Spacing() int                                                { return 0 }
func (delegate) Update(msg tea.Msg, m *list.Model) tea.Cmd                   { return nil }

func newItemDelegate(*delegateKeyMap) delegate { return delegate{} }
