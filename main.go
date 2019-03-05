package main

import (
  "strings"
  "crypto/sha256"
  "strconv"
  "fmt"
  "database/sql"
  "time"
  "io"
  "encoding/hex"
  "net/http"
  "html/template"
  "github.com/labstack/echo"
  _ "github.com/go-sql-driver/mysql"
)

// db変数を定義
var db *sql.DB

// index.html用のtemplate構造体
type Template struct {
  templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
  return t.templates.ExecuteTemplate(w, name, data)
}

// lastfailure にて json を返却するようの構造体
type dbJSON struct {
  Name []string `json:"name"`
  Day []string `json:"day"`
  Reason []string `json:"reason"`
  LastDay []string `json:"lastday"`
  Last string `json:"last"`
}

// Lastfailure に POST される json を格納する構造体
type postJSON struct {
  Line string `json:"line"`
}

// Resettime に POST される from の内容を格納する構造体
type formpostJSON struct {
  Day string
  Reason string
  Target string
  Pass string
}

// / 用の endpoint 処理
func Index(c echo.Context) error {
  return c.Render(http.StatusOK, "index.html", nil)
}

// /api/lastfailure 用の endpoint 処理
func Lastfailure(c echo.Context) error {
  // 今どの障害情報が選択されているかの情報を格納
  u := new(postJSON)
  if err := c.Bind(u); err != nil {
    return err
  }

  // クエリを作成する
  stmtOut, err := db.Prepare(fmt.Sprintf("SELECT day FROM accidentTable WHERE id = ?"))
  // クエリ作成のエラー処理
  if err != nil {
    panic(err.Error())
    }
    // main()関数の最後はdbのコネクションをクローズする
    defer stmtOut.Close()

    // クエリの呼び出し
    var lastfailure_day string
    if err := stmtOut.QueryRow(u.Line).Scan(&lastfailure_day); err != nil {
      fmt.Println(err)
      return nil
    }

    //YYYYMMDD を YYYY MM DD にそれぞれ splitする
    yyyy := lastfailure_day[0:4]
    mm := lastfailure_day[4:6]
    dd := lastfailure_day[6:8]

    yyyyInt, _ := strconv.Atoi(yyyy)
    mmInt, _ := strconv.Atoi(mm)
    ddInt, _ := strconv.Atoi(dd)

    // 選択した障害発生日と現在の日にちを取りだして差分を計算する
    lastfailure_day_format := time.Date(yyyyInt, time.Month(mmInt), ddInt, 0, 0, 0, 0, time.Local)
    nowTime := time.Now()

    unix_lastfailure_day_format := lastfailure_day_format.Unix()
    unix_nowTime := nowTime.Unix()

    interval := (unix_nowTime - unix_lastfailure_day_format) / (60 * 60 * 24)

    // DB から取り出した値を格納する構造体を定義
    var outJSON dbJSON

    // クエリを作成する
    stmtOut2, err := db.Query(fmt.Sprintf("SELECT id, name, day, reason FROM accidentTable"))
    // クエリ作成のエラー処理
    if err != nil {
      panic(err.Error())
      }
      // main()関数の最後はdbのコネクションをクローズする
      defer stmtOut2.Close()

      // クエリの呼び出し
      var id int
      var name string
      var day string
      var reason string

      for stmtOut2.Next() {
        if err := stmtOut2.Scan(&id, &name, &day, &reason); err != nil {
          fmt.Println(err)
          return nil
        }
       //YYYYMMDD を YYYY MM DD にそれぞれ splitする
        yyyy := day[0:4]
        mm := day[4:6]
        dd := day[6:8]

        yyyyInt, _ := strconv.Atoi(yyyy)
        mmInt, _ := strconv.Atoi(mm)
        ddInt, _ := strconv.Atoi(dd)

        // 選択した障害発生日と現在の日にちを取りだして差分を計算する
        lastfailure_day_format := time.Date(yyyyInt, time.Month(mmInt), ddInt, 0, 0, 0, 0, time.Local)
        unix_lastfailure_day_format := lastfailure_day_format.Unix()

        intervalLastDay := (unix_nowTime - unix_lastfailure_day_format) / (60 * 60 * 24)

        // 返却用の JSON に値を格納する
        outJSON.Name = append(outJSON.Name, name)
        outJSON.Day = append(outJSON.Day, day)
        outJSON.Reason = append(outJSON.Reason, reason)
        outJSON.LastDay = append(outJSON.LastDay, strconv.FormatInt(intervalLastDay, 10))
      }

      // 返却用の JSON に値を格納する
      outJSON.Last = strconv.FormatInt(interval, 10)

      // JSON を返却
      return c.JSON(http.StatusOK, outJSON)
}

// /api/resettime 用の endpoint 処理
func Resettime(c echo.Context) error {
  // 今どの障害情報が選択されているかの情報を格納
  u := new(formpostJSON)
  // それぞれの form の値を格納
  u.Reason = c.FormValue("reason")
  u.Target = c.FormValue("target")
  u.Pass = c.FormValue("password")
  // 置換する
  u.Day = strings.Replace(c.FormValue("ResetDate"), "-", "", -1)

  s := sha256.New()
  io.WriteString(s, u.Pass)
  // password チェック
  password := "your password hash"
  converted := sha256.Sum256([]byte(u.Pass))
  inputPass := hex.EncodeToString(converted[:])

  if (inputPass == password) {
    fmt.Println("OK!!!")

    // UPDATEクエリの作成
    stmtIns, err := db.Prepare(fmt.Sprintf("UPDATE accidentTable SET reason = ?, day = ? WHERE (name = ?)"))
    // クエリ作成のエラー処理
    if err != nil {
      panic(err.Error())
    }
    // main()関数の最後はdbのコネクションをクローズする
    defer stmtIns.Close()

    // クエリの呼び出し
    _, err = stmtIns.Exec(u.Reason, u.Day, u.Target)
    if err != nil {
      fmt.Println(err)
      return nil
    }

  } else {
    return echo.NewHTTPError(http.StatusInternalServerError, "Server Error") // エラーを返して ajax から fail したように見せる
  }
  return c.String(http.StatusOK, "UPDATE SUCCESS")
}

// main関数
func main() {
  // dbの初期化
  var err error
  db, err = sql.Open("mysql", "root:password@mysqlhost/database")

  // dbの初期化エラー処理
  if err != nil {
    panic(err.Error())
  }

  // index.htmlのテンプレート作成
  t := &Template{
    templates: template.Must(template.ParseGlob("../../content/static/*.html")),
  }
  // echoを初期化
  e := echo.New()
  // rendererとして作成したテンプレート t を登録
  e.Renderer = t
  // staticのコンテンツに対してecho上でのpathを指定
  e.Static("/static", "../../content/static")

  // index.html 用の endpoint を作成
  e.GET("/", Index)
  // API /api/lastfailure の endpoint を作成
  e.POST("/api/lastfailure", Lastfailure)
  // API /api/resettime の endpoint を作成
  e.POST("/api/resettime", Resettime)

  // Echoのスタート
  e.Start(":3000")

  // main()関数の最後はdbのコネクションをクローズする
  defer db.Close()
}

