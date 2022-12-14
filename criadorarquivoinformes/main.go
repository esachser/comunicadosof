package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/esachser/comunicadosof"
	"github.com/ledongthuc/pdf"
)

func getInformes() []string {
	res, err := http.Get("https://openfinancebrasil.atlassian.net/wiki/plugins/viewsource/viewpagesrc.action?pageId=17367115")
	if err != nil {
		log.Fatal("Erro ao capturar informe: ", err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Status esperado era 200, recebido: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal("Erro inicializando parser HTML: ", err)
	}

	return doc.Find("body > p > a").Map(func(i int, s *goquery.Selection) string {
		return s.AttrOr("href", "")
	})
}

func getInformeText(link string) string {
	res, err := http.Get(link)
	if err != nil {
		log.Fatal("Erro ao capturar informe: ", err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Status esperado era 200, recebido: %d", res.StatusCode)
	}

	builder := strings.Builder{}

	if strings.HasSuffix(link, ".pdf") {
		err = outputPdfText(res.Body, &builder)
	} else {
		err = outputHtmlText(res.Body, &builder)
	}

	return builder.String()
}

func outputHtmlText(f io.Reader, w io.Writer) error {
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return err
	}

	doc.Find("#bodyTable td .mcnTextContent").Each(func(i int, s *goquery.Selection) {
		t := strings.Trim(s.First().Text(), " ")
		if len(t) > 0 && !strings.Contains(t, "Todos os direitos Reservados") &&
			!strings.Contains(t, "Inscreva-se aqui para recebimento periódico do") &&
			!strings.Contains(t, "Não quer mais receber esses e-mails?") &&
			!strings.Contains(t, "Veja este e-mail no seu navegador") &&
			!strings.Contains(t, "Você pode atualizar as suas preferências ou se descadastrar") {
			fmt.Fprintf(w, "%s\n", t)
		}
	})
	return nil
}

// outputPdfText prints out contents of PDF file to stdout.
func outputPdfText(f io.Reader, w io.Writer) error {
	bts, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	pdfReader, err := pdf.NewReader(bytes.NewReader(bts), int64(len(bts)))
	if err != nil {
		return err
	}

	b, err := pdfReader.GetPlainText()
	if err != nil {
		return err
	}
	_, err = io.Copy(w, b)
	return err
}

var rgxNumeroInforme = regexp.MustCompile(`\#\d+`)
var rgxEliminaEspacos = regexp.MustCompile(`(\ \ *)`)
var rgxEliminaEnters = regexp.MustCompile(`(\n[\ \t]*)`)
var rgxEliminaEnters2 = regexp.MustCompile(`(\n+)`)

func main() {
	enc := json.NewEncoder(os.Stdout)
	fmt.Fprintf(os.Stdout, "[\n")
	for _, link := range getInformes() {
		informeStr := getInformeText(link)
		informeStr = rgxEliminaEspacos.ReplaceAllString(informeStr, " ")
		informeStr = rgxEliminaEnters.ReplaceAllString(informeStr, "\n")
		informeStr = rgxEliminaEnters2.ReplaceAllString(informeStr, "\n")
		// Captura numero do informe
		numeroInforme := rgxNumeroInforme.FindString(informeStr)
		// fmt.Println(informeStr)
		if len(numeroInforme) > 1 {
			inf := comunicadosof.Informe{Link: link, Numero: numeroInforme[1:], Informe: informeStr}
			enc.Encode(inf)
			fmt.Fprintf(os.Stdout, ",\n")
		}
	}
	fmt.Fprintf(os.Stdout, "]\n")
}
