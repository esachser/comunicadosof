package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/esachser/comunicadosof"
)

func getInformeText(link string) (string, error) {
	res, err := http.Get(link)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Status code inesperado: %d", res.StatusCode)
	}

	builder := strings.Builder{}
	err = outputHtmlText(res.Body, &builder)

	return builder.String(), nil
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

func main() {
	flag.Usage = func() {
		_, filename := path.Split(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <url-informe>\n", filename)
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(0)
	}
	link := flag.Arg(0)

	informeStr, err := getInformeText(link)
	if err != nil {
		log.Fatalf("Erro ao processar informe: %v", err)
	}
	numeroInforme := rgxNumeroInforme.FindString(informeStr)
	if len(numeroInforme) < 1 {
		log.Fatal("Erro ao processar o número do informe")
	}
	fmt.Print(informeStr)
	inf := comunicadosof.Informe{Link: link, Numero: numeroInforme[1:], Informe: informeStr}

	obj, err := s3sess.GetObject(&s3.GetObjectInput{Bucket: aws.String("informesofbr"), Key: aws.String("informes.json")})
	if err != nil {
		log.Fatalf("Erro ao buscar objeto no s3: %v", err)
	}
	defer obj.Body.Close()
	fmt.Println("Informes capturados com sucesso")
	dec := json.NewDecoder(obj.Body)
	ifs := []json.RawMessage{}
	err = dec.Decode(&ifs)
	if err != nil {
		log.Fatalf("Erro ao fazer o parse do JSON: %v", err)
	}

	result := &strings.Builder{}
	fmt.Fprintln(result, "[")

	bts, _ := json.Marshal(inf)
	fmt.Fprintf(result, "%s,\n", bts)

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
		log.Fatalf("Erro ao inserir objeto: %v", err)
	}

	log.Print("Sucesso ao inserir objeto")
	log.Printf("%v", r)
}
