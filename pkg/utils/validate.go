package utils

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type (
	Duration string
)

const (
	FiveMinutes   Duration = "5m"
	ThirtyMinutes Duration = "30m"
	OneHour       Duration = "1h"
	FourHours     Duration = "4h"
	OneDay        Duration = "1d"
	OneWeek       Duration = "1w"
	OneMonth      Duration = "1M"
)

var (
	_2021_TIMESTAMP = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC).UnixMicro()
)

// CustomValidator is type setting of third party validator
type CustomValidator struct {
	validator *validator.Validate
}

// Init validator
func NewValidator() *CustomValidator {
	cv := &CustomValidator{
		validator: validator.New(),
	}

	cv.validator.RegisterValidation("price", func(fl validator.FieldLevel) bool {
		if price, ok := fl.Field().Interface().(float64); ok {
			return price > 0
		}
		return false
	})

	cv.validator.RegisterValidation("candlestick_interval", func(fl validator.FieldLevel) bool {
		if interval, ok := fl.Field().Interface().(Duration); ok {
			switch interval {
			case FiveMinutes, ThirtyMinutes, OneHour, FourHours, OneDay, OneWeek, OneMonth:
				return true
			}
		}
		return false
	})

	cv.validator.RegisterValidation("micro_timestamp", func(fl validator.FieldLevel) bool {
		// Nano timestamp must be greater than 2021-01-01 00:00:00

		if timestamp, ok := fl.Field().Interface().(int64); ok {
			if timestamp == -1 { // -1 is a special value to get the latest candlestick
				return true
			}

			if timestamp < 0 || timestamp > time.Now().UnixMicro() || timestamp < _2021_TIMESTAMP {
				return false
			}
			return timestamp%int64(time.Microsecond) == 0
		}
		return false
	})

	cv.validator.RegisterValidation("valid_symbol", func(fl validator.FieldLevel) bool {
		return true
	})

	cv.validator.RegisterValidation("valid_wallet_type", func(fl validator.FieldLevel) bool {
		return true
	})

	return cv
}

type ValidationError struct {
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata"`
}

func (ve ValidationError) Error() string {
	return ve.Message
}

func (cv *CustomValidator) Validate(i any) error {
	if err := cv.validator.Struct(i); err != nil {
		validateErrs, ok := err.(validator.ValidationErrors)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		firstErr := validateErrs[0]
		errMsg := translateError(firstErr)

		validationErr := &ValidationError{
			Message: errMsg,
			Metadata: map[string]any{
				"field":     firstErr.Field(),
				"namespace": firstErr.Namespace(),
				"type":      firstErr.Type().String(),
				"tag":       firstErr.Tag(),
				"param":     firstErr.Param(),
			},
		}
		return echo.NewHTTPError(http.StatusBadRequest, validationErr)
	}
	return nil
}

// See list of tag here https://github.com/go-playground/validator?tab=readme-ov-file#baked-in-validations
// TODO: Add more validation error message
func translateError(validateErr validator.FieldError) string {
	tag := validateErr.Tag()
	field := pascalCaseToWords(validateErr.Field())

	var msg string = ""
	switch tag {
	// Baked-in validation
	case "alpha":
		msg = fmt.Sprintf("%s must contain only letters", field)
	case "alphanum":
		msg = fmt.Sprintf("%s must contain only letters and numbers", field)
	case "alphanumunicode":
		msg = fmt.Sprintf("%s must contain only letters, numbers, and unicode characters", field)
	case "alphaunicode":
		msg = fmt.Sprintf("%s must contain only letters and unicode characters", field)
	case "ascii":
		msg = fmt.Sprintf("%s must contain only ascii characters", field)
	case "contains":
		msg = fmt.Sprintf("%s must contain %s", field, validateErr.Param())
	case "required":
		msg = fmt.Sprintf("%s is required", field)
	case "number", "cidr":
		msg = fmt.Sprintf("%s must be a %s", field, tag)
	case "email", "url", "uri", "uuid":
		msg = fmt.Sprintf("%s must be an %s", field, tag)
	case "min":
		switch validateErr.Type().Name() {
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			msg = fmt.Sprintf("%s must be greater than or equal to %s", field, validateErr.Param())
		case "string":
			msg = fmt.Sprintf("%s must be at least %s characters long", field, validateErr.Param())
		case "slice", "array":
			msg = fmt.Sprintf("%s must have at least %s items", field, validateErr.Param())
		}
	case "max":
		switch validateErr.Type().Name() {
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			msg = fmt.Sprintf("%s must be less than or equal to %s", field, validateErr.Param())
		case "string":
			msg = fmt.Sprintf("%s must be at most %s characters long", field, validateErr.Param())
		case "slice", "array":
			msg = fmt.Sprintf("%s must have at most %s items", field, validateErr.Param())
		}
	case "oneof":
		params := strings.Split(validateErr.Param(), " ")
		paramMsg := ""
		for i, param := range params {
			if i == len(params)-1 {
				paramMsg += fmt.Sprintf("or %s", param)
			} else {
				paramMsg += fmt.Sprintf("%s, ", param)
			}
		}
		msg = fmt.Sprintf("%s must be one of %s", field, paramMsg)
	case "gt":
		msg = fmt.Sprintf("%s must be greater than %s", field, validateErr.Param())
	case "gte":
		msg = fmt.Sprintf("%s must be greater than or equal to %s", field, validateErr.Param())
	case "lt":
		msg = fmt.Sprintf("%s must be less than %s", field, validateErr.Param())
	case "lte":
		msg = fmt.Sprintf("%s must be less than or equal to %s", field, validateErr.Param())
	case "eq":
		msg = fmt.Sprintf("%s must be equal to %s", field, validateErr.Param())
	case "ne":
		msg = fmt.Sprintf("%s must not be equal to %s", field, validateErr.Param())
	case "gtfield":
		msg = fmt.Sprintf("%s must be greater than %s", field, strings.ToLower(validateErr.Param()))
	case "gtefield":
		msg = fmt.Sprintf("%s must be greater than or equal to %s", field, strings.ToLower(validateErr.Param()))
	case "ltfield":
		msg = fmt.Sprintf("%s must be less than %s", field, strings.ToLower(validateErr.Param()))
	case "ltefield":
		msg = fmt.Sprintf("%s must be less than or equal to %s", field, strings.ToLower(validateErr.Param()))
	case "eqfield":
		msg = fmt.Sprintf("%s must be equal to %s", field, strings.ToLower(validateErr.Param()))
	case "nefield":
		msg = fmt.Sprintf("%s must not be equal to %s", field, strings.ToLower(validateErr.Param()))
	case "required_with":
		msg = fmt.Sprintf("%s is required when %s is present", field, validateErr.Param())
	case "required_without":
		msg = fmt.Sprintf("%s is required when %s is not present", field, validateErr.Param())
	case "required_if":
		params := strings.Split(validateErr.Param(), " ")
		comparedField := strings.ToLower(params[0])
		compareValues := params[1:]
		compareValuesMsg := ""
		if len(compareValues) == 1 {
			compareValuesMsg = compareValues[0]
		} else {
			for i, fieldValue := range compareValues {
				if i == len(compareValues)-1 {
					compareValuesMsg += fmt.Sprintf("or %s", fieldValue)
				} else {
					compareValuesMsg += fmt.Sprintf("%s, ", fieldValue)
				}
			}
		}
		msg = fmt.Sprintf("%s is required when %s is %s", field, comparedField, compareValuesMsg)
	}
	if len(msg) == 0 {
		msg = validateErr.Error()
	}

	return msg
}

func pascalCaseToWords(s string) string {
	var words []rune
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			words = append(words, ' ')
			r += 'a' - 'A'
		}
		words = append(words, r)
	}
	return string(words)
}

func BindNValidate(c echo.Context, i any) error {
	if err := c.Bind(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(i); err != nil {
		return err
	}
	return nil
}
