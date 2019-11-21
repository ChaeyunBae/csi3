package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/digital-idea/ditime"
	"gopkg.in/mgo.v2"
)

// handleImportExcel 함수는 엑셀파일을 Import 하는 페이지 이다.
func handleImportExcel(w http.ResponseWriter, r *http.Request) {
	ssid, err := GetSessionID(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}
	if ssid.AccessLevel == 0 {
		http.Redirect(w, r, "/invalidaccess", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	type recipe struct {
		User
		SessionID string
		Devmode   bool
	}
	rcp := recipe{}
	rcp.Devmode = *flagDevmode
	rcp.SessionID = ssid.ID
	session, err := mgo.Dial(*flagDBIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()
	rcp.User, err = getUser(session, ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 기존 Temp 경로 내부 .xlsx 데이터를 삭제한다.
	tmp, err := userTemppath(ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = RemoveXLSX(tmp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = TEMPLATES.ExecuteTemplate(w, "importexcel", rcp)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleUploadExcel(w http.ResponseWriter, r *http.Request) {
	ssid, err := GetSessionID(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}
	if ssid.AccessLevel == 0 {
		http.Redirect(w, r, "/invalidaccess", http.StatusSeeOther)
		return
	}
	// dropzone setting
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println(err)
	}
	defer file.Close()
	mimeType := header.Header.Get("Content-Type")
	switch mimeType {
	case "text/csv":
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
			return
		}
		tmp, err := userTemppath(ssid.ID)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
		}
		path := tmp + "/" + header.Filename // 업로드한 파일 리스트를 불러오기 위해 뒤에 붙는 Unixtime을 제거한다.
		err = ioutil.WriteFile(path, data, 0666)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
			return
		}
		fmt.Println(path)
	case "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": // MS-Excel, Google & Libre Excel
		data, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
			return
		}
		tmp, err := userTemppath(ssid.ID)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
		}
		path := tmp + "/" + header.Filename // 업로드한 파일 리스트를 불러오기 위해 뒤에 붙는 Unixtime을 제거한다.
		err = ioutil.WriteFile(path, data, 0666)
		if err != nil {
			fmt.Fprintf(w, "%v", err)
			return
		}
	default:
		// 지원하지 않는 파일. 저장하지 않는다.
		log.Printf("Not support: %s", mimeType)
	}
}

// handleReportExcel 함수는 excel 파일을 체크하고 분석 보고서로 Redirection 한다.
func handleReportExcel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Get Only", http.StatusMethodNotAllowed)
		return
	}
	ssid, err := GetSessionID(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}
	if ssid.AccessLevel == 0 {
		http.Redirect(w, r, "/invalidaccess", http.StatusSeeOther)
		return
	}
	session, err := mgo.Dial(*flagDBIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()
	// 파일네임을 구한다.
	tmppath, err := userTemppath(ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// .xlsx 파일을 읽는다.
	xlsxs, err := GetXLSX(tmppath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(xlsxs) != 1 {
		http.Redirect(w, r, "/importexcel", http.StatusSeeOther)
		return
	}
	f, err := excelize.OpenFile(xlsxs[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type recipe struct {
		Project   string
		Filename  string
		Sheet     string
		Overwrite bool
		Rows      []Excelrow
		User
		SessionID string
		Devmode   bool
		SearchOption
		Errornum    int
		Projectlist []string
	}
	rcp := recipe{}
	rcp.Sheet = "Sheet1"
	rcp.Projectlist, err = OnProjectlist(session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rows []Excelrow
	excelRows := f.GetRows(rcp.Sheet)
	if len(excelRows) == 0 {
		http.Error(w, rcp.Sheet+"값이 비어있습니다.", http.StatusBadRequest)
		return
	}
	for _, line := range excelRows {
		if len(line) != 15 {
			http.Error(w, "약속된 Cell 갯수가 다릅니다", http.StatusBadRequest)
			return
		}
		if line[0] == "샷네임" {
			continue
		}
		row := Excelrow{}
		row.Name = line[0]             // item name
		row.Shottype = line[1]         // shottype 2d,3d
		row.Note = line[2]             // 작업내용
		row.Comment = line[3]          // 수정사항
		row.Link = line[4]             // 링크자료(제목:경로)
		row.Ddline3D = line[5]         // 3D마감
		row.Ddline2D = line[6]         // 2D마감
		row.Findate = line[7]          // FIN날짜
		row.Finver = line[8]           // FIN버전
		row.Tags = line[9]             // 태그
		row.Rnum = line[10]            // 롤넘버
		row.HandleIn = line[11]        // 핸들IN
		row.HandleOut = line[12]       // 핸들OUT
		row.JustTimecodeIn = line[13]  // JUST타임코드IN
		row.JustTimecodeOut = line[14] // JUST타임코드OUT
		row.checkerror()               // 각 값을 에러체크한다.
		rcp.Errornum += row.Errornum
		rows = append(rows, row)
	}
	rcp.SessionID = ssid.ID
	rcp.Devmode = *flagDevmode
	rcp.SearchOption = handleRequestToSearchOption(r)
	rcp.User, err = getUser(session, ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rcp.Rows = rows
	err = TEMPLATES.ExecuteTemplate(w, "reportexcel", rcp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

// handleExcelSubmit 함수는 excel 파일을 전송한다.
func handleExcelSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Post Only", http.StatusMethodNotAllowed)
		return
	}
	ssid, err := GetSessionID(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}
	if ssid.AccessLevel == 0 {
		http.Redirect(w, r, "/invalidaccess", http.StatusSeeOther)
		return
	}
	session, err := mgo.Dial(*flagDBIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()
	// 파일네임을 구한다.
	tmppath, err := userTemppath(ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// .xlsx 파일을 읽는다.
	xlsxs, err := GetXLSX(tmppath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(xlsxs) != 1 {
		http.Redirect(w, r, "/importexcel", http.StatusSeeOther)
		return
	}
	f, err := excelize.OpenFile(xlsxs[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type ErrorItem struct {
		Name  string
		Error string
	}
	type recipe struct {
		Filename string
		Sheet    string
		User
		SessionID string
		Devmode   bool
		SearchOption
		ErrorItems []ErrorItem
	}
	rcp := recipe{}
	rcp.SessionID = ssid.ID
	rcp.Devmode = *flagDevmode
	rcp.SearchOption = handleRequestToSearchOption(r)
	rcp.User, err = getUser(session, ssid.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rcp.Sheet = "Sheet1"
	project := r.FormValue("project")
	overwrite := str2bool(r.FormValue("overwrite"))

	excelRows := f.GetRows(rcp.Sheet)
	if len(excelRows) == 0 {
		http.Error(w, rcp.Sheet+"값이 비어있습니다.", http.StatusBadRequest)
		return
	}
	for _, line := range excelRows {
		if len(line) != 15 {
			http.Error(w, "약속된 Cell 갯수가 다릅니다", http.StatusBadRequest)
			return
		}
		if line[0] == "샷네임" {
			continue
		}
		name := line[0]     // item name
		shottype := line[1] // shottype 2d,3d
		if shottype != "" {
			err := SetShotType(session, project, name, shottype)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		note := line[2] // 작업내용
		if note != "" {
			_, err = SetNote(session, project, name, ssid.ID, note, overwrite)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		comment := line[3] // 수정사항
		if comment != "" {
			err = AddComment(session, project, name, ssid.ID, time.Now().Format(time.RFC3339), comment)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		sources := line[4] // 소스자료(제목:경로)
		if sources != "" {
			for _, s := range strings.Split(sources, "\n") {
				source := strings.Split(s, ":")
				title := source[0]
				path := source[1]
				err = AddSource(session, project, name, ssid.ID, title, path)
				if err != nil {
					rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
					continue
				}
			}
		}

		ddline3d := line[5] // 3D마감
		if ddline3d != "" {
			date, err := ditime.ToFullTime(19, ddline3d)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
			err = SetDeadline3D(session, project, name, date)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		ddline2d := line[6] // 2D마감
		if ddline2d != "" {
			date, err := ditime.ToFullTime(19, ddline2d)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
			err = SetDeadline2D(session, project, name, date)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		findate := line[7] // FIN날짜
		if findate != "" {
			date, err := ditime.ToFullTime(19, findate)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
			err = SetFindate(session, project, name, date)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		finver := line[8] // FIN버전
		if finver != "" {
			err = SetFinver(session, project, name, finver)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}

		tags := line[9] // 태그
		if tags != "" {
			err = SetTags(session, project, name, strings.Split(tags, ","))
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		rnum := line[10] // 롤넘버
		if rnum != "" {
			err = SetRnum(session, project, name, rnum)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		handleIn := line[11] // 핸들IN
		if handleIn != "" {
			num, err := strconv.Atoi(handleIn)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
			err = SetFrame(session, project, name, "handlein", num)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		handleOut := line[12] // 핸들OUT
		if handleOut != "" {
			num, err := strconv.Atoi(handleOut)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
			err = SetFrame(session, project, name, "handleout", num)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		justTimecodeIn := line[13] // JUST타임코드IN
		if justTimecodeIn != "" {
			err = SetJustTimecodeIn(session, project, name, justTimecodeIn)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
		justTimecodeOut := line[14] // JUST타임코드OUT
		if justTimecodeOut != "" {
			err = SetJustTimecodeIn(session, project, name, justTimecodeOut)
			if err != nil {
				rcp.ErrorItems = append(rcp.ErrorItems, ErrorItem{Name: name, Error: err.Error()})
				continue
			}
		}
	}
	err = TEMPLATES.ExecuteTemplate(w, "resultexcel", rcp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
