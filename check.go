package main

import (
	"errors"
	"os"
	"regexp"
	"strings"
)

// 해상도값 정규식 : 2048x1080 형태
var regexpImageSize = regexp.MustCompile(`\d{2,5}[xX]\d{2,5}$`)

// 샷네임값 정규식 : SS_0010 형태
var regexpShotname = regexp.MustCompile(`^[a-zA-Z0-9]+_[a-zA-Z0-9]+$`)

// 에셋 네임값 정규식 : stone01 형태
var regexpAssetname = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// 롤미디어 정규식 : 00_A03C001_180113_A001 형태
var regexpRollMedia = regexp.MustCompile(`^\d+_[A-Z0-9]+_\d+_[A-Z0-9]+$`)

// Alexa 카메라의 형태 : N_AAAACCCC_YYMMDD_RRRR
// - N : order
// - AAAACCCC : reel name
// - YYMMDD : 년,월,일
// - RRRR : Unique hex code
//   -  참고 : embedded 네임은 AAAARRRR 형태이다.
//
// Red카메라의 형태 : C RRR TTT dd mm HH NNN
// - C : Camera letter, from A to Z
// - RRR : three digit reel number
// - TTT : three digit take number
// - dd mm : date and  month
// - HH : unique hex code that ensures another file won't have the exact same name
// - NNN : spanned clip number

// str2bool은 받아들인 문자열이 "true"나 "True"라면 참을, 그 외에는 거짓을 반환한다.
func str2bool(str string) bool {
	if str == "true" || str == "True" {
		return true
	}
	return false
}

// mov경로를 체크하는 함수이다.
func isMov(path string) bool {
	// 빈 문자열인지 체크
	if path == "" {
		return false
	}
	// .mov로 끝나는지 체크
	if !strings.HasSuffix(strings.ToLower(path), ".mov") {
		return false
	}
	// 경로가 존재하는지 체크
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// 에셋타입이 유효한지 체크하는 함수이다.
func validAssettype(assettype string) error {
	switch assettype {
	case "char", "comp", "env", "fx", "matte", "plant", "prop", "vehicle", "concept":
		return nil
	}
	return errors.New(assettype + "이름을 에셋타입으로 사용할 수 없습니다")
}

// validShottype 함수는 샷타입이 유효한지 체크하는 함수이다.
func validShottype(shottype string) error {
	switch shottype {
	case "2d", "3d":
		return nil
	}
	return errors.New(shottype + "이름을 샷타입으로 사용할 수 없습니다")
}

// Task 값이 유효한지 체크하는 함수이다.
func validTask(inputTask string) error {
	for _, task := range TASKS {
		// 아래 테스크 이름을 체크하는 함수는 역사가 만들어낸 산물이며 천천히 제거되어야 한다.
		if renameTask(task) == renameTask(inputTask) {
			return nil
		}
	}
	return errors.New(inputTask + "이름을 task 이름으로 사용할 수 없습니다")
}

func renameTask(task string) string {
	// fursim은 회사에서 사용하고 있는 특수한 Task이다.
	// 샷 작업은 fursim이고 에셋작업은 fur로 불린다.(작업회의중)
	// 아래 테스크 이름은 역사가 만들어낸 산물이며 천천히 제거되어야 한다.
	switch strings.ToLower(task) {
	case "fursim":
		return "fur"
	case "lookdev", "look":
		return "light"
	case "rig":
		return "sim"
	default:
		return strings.ToLower(task)
	}
}
