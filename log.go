package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/now"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 只能输出结构化日志，但是性能要高于 SugaredLogger
var logger *zap.Logger

// 可以输出 结构化日志、非结构化日志。性能茶语 zap.Logger，具体可见上面的的单元测试
var sugarLogger *zap.SugaredLogger

func getWriter(filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    10,    //最大M数，超过则切割
		MaxBackups: 5,     //最大文件保留数，超过就删除最老的日志文件
		MaxAge:     30,    //保存30天
		Compress:   false, //是否压缩
	}
}

const logLayout = "2006-01-02_150405"

func init() {
	nowTime := time.Now()
	InitLog(fmt.Sprintf("log/INFO.%s.log", time.Now().Format(logLayout)), fmt.Sprintf("log/ERROR.%s.log", time.Now().Format(logLayout)), zap.InfoLevel)
	go func() {
		for {
			time.Sleep(now.BeginningOfHour().Add(time.Hour).Sub(nowTime))
			InitLog(fmt.Sprintf("INFO.%s.log", time.Now().Format(logLayout)), fmt.Sprintf("ERROR.%s.log", time.Now().Format(logLayout)), zap.InfoLevel)
		}
	}()
}

// 初始化日志 logger
func InitLog(logPath, errPath string, logLevel zapcore.Level) {
	config := zapcore.EncoderConfig{
		MessageKey:   "msg",                       //结构化（json）输出：msg的key
		LevelKey:     "level",                     //结构化（json）输出：日志级别的key（INFO，WARN，ERROR等）
		TimeKey:      "ts",                        //结构化（json）输出：时间的key（INFO，WARN，ERROR等）
		CallerKey:    "file",                      //结构化（json）输出：打印日志的文件对应的Key
		EncodeLevel:  zapcore.CapitalLevelEncoder, //将日志级别转换成大写（INFO，WARN，ERROR等）
		EncodeCaller: zapcore.ShortCallerEncoder,  //采用短文件路径编码输出（test/main.go:14 ）
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		}, //输出的时间格式
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		}, //
	}
	//自定义日志级别：自定义Info级别
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.WarnLevel && lvl >= logLevel
	})

	//自定义日志级别：自定义Warn级别
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel && lvl >= logLevel
	})

	// 获取io.Writer的实现
	infoWriter := getWriter(logPath)
	warnWriter := getWriter(errPath)

	// 实现多个输出
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.AddSync(infoWriter), infoLevel),                         //将info及以下写入logPath，NewConsoleEncoder 是非结构化输出
		zapcore.NewCore(zapcore.NewConsoleEncoder(config), zapcore.AddSync(warnWriter), warnLevel),                         //warn及以上写入errPath
		zapcore.NewCore(zapcore.NewJSONEncoder(config), zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), logLevel), //同时将日志输出到控制台，NewJSONEncoder 是结构化输出
	)
	logger = zap.New(core)
	sugarLogger = logger.Sugar()
}

const contextLogIdKey = "log_id_114514"

func NewContextWithLogId(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextLogIdKey, uuid.New().String())
}

func getLogId(ctx context.Context) string {
	str, _ := ctx.Value(contextLogIdKey).(string)
	return str
}

func buildField(ctx context.Context, fields ...zap.Field) []zap.Field {
	_, file, line, _ := runtime.Caller(2)
	return append(fields, zap.String("logid", getLogId(ctx)), zap.String("file", fmt.Sprintf("%s:%d", file, line)))
}

func CtxInfo(ctx context.Context, msg string, fields ...zap.Field) {
	logger.Info(msg, buildField(ctx, fields...)...)
}

func CtxError(ctx context.Context, msg string, fields ...zap.Field) {
	logger.Error(msg, buildField(ctx, fields...)...)
}

func CtxPanic(ctx context.Context, msg string, fields ...zap.Field) {

	logger.Panic(msg, buildField(ctx, fields...)...)
}

func CtxFatal(ctx context.Context, msg string, fields ...zap.Field) {
	logger.Fatal(msg, buildField(ctx, fields...)...)
}

func CtxWarn(ctx context.Context, msg string, fields ...zap.Field) {
	logger.Warn(msg, buildField(ctx, fields...)...)
}

func CtxRecover(ctx context.Context) {
	if r := recover(); r != nil {
		CtxPanic(ctx, "panic happened!", zap.Any("reason", r), zap.String("stack", string(debug.Stack())))
	}
}
