package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/esachser/comunicadosof"
)

func getInformes2() ([]string, error) {
	res, err := http.Get("https://openfinancebrasil.atlassian.net/wiki/plugins/viewsource/viewpagesrc.action?pageId=17367115")
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code inesperado: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	return doc.Find("body > table > tbody > tr > td > p > a").Map(func(i int, s *goquery.Selection) string {
		return s.AttrOr("href", "")
	}), nil
}

func getInformes() ([]string, error) {
	res, err := http.Get("https://openfinancebrasil.atlassian.net/wiki/plugins/viewsource/viewpagesrc.action?pageId=17367115")
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code inesperado: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	return doc.Find("body > p > a").Map(func(i int, s *goquery.Selection) string {
		return s.AttrOr("href", "") + " " + s.Parent().Text()
	}), nil
}

func getInformeText(link string) (string, error) {
	res, err := http.Get(link)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code inesperado: %d", res.StatusCode)
	}

	builder := strings.Builder{}
	err = outputHtmlText(res.Body, &builder)

	return builder.String(), err
}

func getInformeTitleAndText(link string) (string, string, error) {
	res, err := http.Get(link)
	if err != nil {
		return "", "", err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("status code inesperado: %d", res.StatusCode)
	}

	builder := strings.Builder{}
	bts, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", err
	}
	err = outputHtmlText(bytes.NewReader(bts), &builder)
	if err != nil {
		return "", "", err
	}
	text := builder.String()

	builder = strings.Builder{}
	err = outputHtmlTitle(bytes.NewReader(bts), &builder)
	if err != nil {
		return "", "", err
	}
	title := builder.String()

	return title, text, nil
}

func outputHtmlTitle(f io.Reader, w io.Writer) error {
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return err
	}

	doc.Find("head > title").Each(func(i int, s *goquery.Selection) {
		fmt.Fprint(w, s.Text())
	})
	return nil
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

var client = session.Must(session.NewSession())
var s3sess = s3.New(client)

var rgxNumeroInforme = regexp.MustCompile(`\#\d+`)

func getobject() (string, error) {
	fmt.Println("Iniciando execução")
	obj, err := s3sess.GetObject(&s3.GetObjectInput{Bucket: aws.String("informesofbr"), Key: aws.String("informes.json")})
	if err != nil {
		return "", err
	}
	defer obj.Body.Close()
	fmt.Println("Informes capturados com sucesso")
	dec := json.NewDecoder(obj.Body)
	ifs := []json.RawMessage{}
	err = dec.Decode(&ifs)
	if err != nil {
		return "", err
	}
	if len(ifs) == 0 {
		return "", nil
	}

	firstInforme := ifs[0]
	informe := comunicadosof.Informe{}
	err = json.Unmarshal(firstInforme, &informe)
	if err != nil {
		return "", err
	}

	informeNumero, _ := strconv.Atoi(informe.Numero)
	fmt.Printf("Último informe presente: %d\n", informeNumero)

	informesList, err := getInformes2()
	if err != nil {
		return "", nil
	}

	fmt.Println("Lista de links de informes capturado com sucesso")

	ctEscritos := 0
	result := &strings.Builder{}
	fmt.Fprintln(result, "[")
	for _, inf := range informesList {
		fmt.Println(inf)
		// splt := strings.Split(inf, " ")
		link := inf

		fmt.Printf("Capturando informe em %s\n", link)
		text, infText, err := getInformeTitleAndText(link)
		if err != nil {
			return "", err
		}

		fmt.Printf("Informe: %s\n", text)

		ns := rgxNumeroInforme.FindString(text)
		if len(ns) < 2 {
			return "", errors.New("não foi possível processar os informes")
		}
		n, _ := strconv.Atoi(ns[1:])
		fmt.Printf("Processando informe #%d\n", n)
		if n <= informeNumero {
			break
		}
		if len(infText) == 0 {
			fmt.Printf("Capturando dados do informe #%d em %s\n", n, link)
			infText, err = getInformeText(link)
			if err != nil {
				return "", err
			}
		}

		bts, _ := json.Marshal(comunicadosof.Informe{Link: link, Numero: ns[1:], Informe: infText})
		fmt.Fprintf(result, "%s,\n", bts)
		ctEscritos += 1
	}

	if ctEscritos > 0 {
		for i := range ifs {
			fmt.Fprintf(result, "%s", ifs[i])
			if i == len(ifs)-1 {
				fmt.Fprintf(result, "\n")
			} else {
				fmt.Fprintf(result, ",\n")
			}
		}
		fmt.Fprintf(result, "]\n")

		// Escrever no bucket S3 com PutObject
		newObj := s3.PutObjectInput{
			ACL:                aws.String("public-read"),
			Body:               strings.NewReader(result.String()),
			Bucket:             aws.String("informesofbr"),
			BucketKeyEnabled:   obj.BucketKeyEnabled,
			CacheControl:       obj.CacheControl,
			ContentDisposition: obj.ContentDisposition,
			ContentEncoding:    obj.ContentEncoding,
			ContentLanguage:    obj.ContentLanguage,
			ContentType:        obj.ContentType,
			Key:                aws.String("informes.json"),
		}

		r, err := s3sess.PutObject(&newObj)
		if err != nil {
			return "", err
		}

		return r.String(), nil
	}

	return "Não precisou atualizar", nil
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(getobject)
}
