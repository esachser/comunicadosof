package main

import (
	"strings"
	"testing"
)

func TestGetInformes(t *testing.T) {
	inf, err := getInformes()

	if err != nil {
		t.Fatal(err)
	}

	t.Log(inf)
}

func TestGetInformes2(t *testing.T) {
	inf, err := getInformes2()

	if err != nil {
		t.Fatal(err)
	}

	t.Log(inf[0])
}

func TestGetInformesWithTextAndTitle(t *testing.T) {
	inf, err := getInformes2()

	if err != nil {
		t.Fatal(err)
	}

	t.Log(inf)

	t.Log(getInformeTitleAndText(inf[0]))

	splt := strings.Split(inf[0], " ")
	link := splt[0]

	t.Log(link)
}
