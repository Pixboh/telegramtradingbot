package tgbot

import (
	"errors"
	"fmt"
)

/**
Error Codes

GetLastError() - the function that returns codes of error. Code constants of errors are determined in stderror.mqh file. To draw the text messages use the ErrorDescription() function described in the stdlib.mqh file.


Error codes returned from a trade server or client terminal:

Constant	Value	Description
ERR_NO_ERROR	0	No error returned.
ERR_NO_RESULT	1	No error returned, but the result is unknown.
ERR_COMMON_ERROR	2	Common error.
ERR_INVALID_TRADE_PARAMETERS	3	Invalid trade parameters.
ERR_SERVER_BUSY	4	Trade server is busy.
ERR_OLD_VERSION	5	Old version of the client terminal.
ERR_NO_CONNECTION	6	No connection with trade server.
ERR_NOT_ENOUGH_RIGHTS	7	Not enough rights.
ERR_TOO_FREQUENT_REQUESTS	8	Too frequent requests.
ERR_MALFUNCTIONAL_TRADE	9	Malfunctional trade operation.
ERR_ACCOUNT_DISABLED	64	Account disabled.
ERR_INVALID_ACCOUNT	65	Invalid account.
ERR_TRADE_TIMEOUT	128	Trade timeout.
ERR_INVALID_PRICE	129	Invalid price.
ERR_INVALID_STOPS	130	Invalid stops.
ERR_INVALID_TRADE_VOLUME	131	Invalid trade volume.
ERR_MARKET_CLOSED	132	Market is closed.
ERR_TRADE_DISABLED	133	Trade is disabled.
ERR_NOT_ENOUGH_MONEY	134	Not enough money.
ERR_PRICE_CHANGED	135	Price changed.
ERR_OFF_QUOTES	136	Off quotes.
ERR_BROKER_BUSY	137	Broker is busy.
ERR_REQUOTE	138	Requote.
ERR_ORDER_LOCKED	139	Order is locked.
ERR_LONG_POSITIONS_ONLY_ALLOWED	140	Long positions only allowed.
ERR_TOO_MANY_REQUESTS	141	Too many requests.
ERR_TRADE_MODIFY_DENIED	145	Modification denied because an order is too close to market.
ERR_TRADE_CONTEXT_BUSY	146	Trade context is busy.
ERR_TRADE_EXPIRATION_DENIED	147	Expirations are denied by broker.
ERR_TRADE_TOO_MANY_ORDERS	148	The amount of opened and pending orders has reached the limit set by a broker.

MQL4 run time error codes:

Constant	Value	Description
ERR_NO_MQLERROR	4000	No error.
ERR_WRONG_FUNCTION_POINTER	4001	Wrong function pointer.
ERR_ARRAY_INDEX_OUT_OF_RANGE	4002	Array index is out of range.
ERR_NO_MEMORY_FOR_FUNCTION_CALL_STACK	4003	No memory for function call stack.
ERR_RECURSIVE_STACK_OVERFLOW	4004	Recursive stack overflow.
ERR_NOT_ENOUGH_STACK_FOR_PARAMETER	4005	Not enough stack for parameter.
ERR_NO_MEMORY_FOR_PARAMETER_STRING	4006	No memory for parameter string.
ERR_NO_MEMORY_FOR_TEMP_STRING	4007	No memory for temp string.
ERR_NOT_INITIALIZED_STRING	4008	Not initialized string.
ERR_NOT_INITIALIZED_ARRAYSTRING	4009	Not initialized string in an array.
ERR_NO_MEMORY_FOR_ARRAYSTRING	4010	No memory for an array string.
ERR_TOO_LONG_STRING	4011	Too long string.
ERR_REMAINDER_FROM_ZERO_DIVIDE	4012	Remainder from zero divide.
ERR_ZERO_DIVIDE	4013	Zero divide.
ERR_UNKNOWN_COMMAND	4014	Unknown command.
ERR_WRONG_JUMP	4015	Wrong jump.
ERR_NOT_INITIALIZED_ARRAY	4016	Not initialized array.
ERR_DLL_CALLS_NOT_ALLOWED	4017	DLL calls are not allowed.
ERR_CANNOT_LOAD_LIBRARY	4018	Cannot load library.
ERR_CANNOT_CALL_FUNCTION	4019	Cannot call function.
ERR_EXTERNAL_EXPERT_CALLS_NOT_ALLOWED	4020	EA function calls are not allowed.
ERR_NOT_ENOUGH_MEMORY_FOR_RETURNED_STRING	4021	Not enough memory for a string returned from a function.
ERR_SYSTEM_BUSY	4022	System is busy.
ERR_INVALID_FUNCTION_PARAMETERS_COUNT	4050	Invalid function parameters count.
ERR_INVALID_FUNCTION_PARAMETER_VALUE	4051	Invalid function parameter value.
ERR_STRING_FUNCTION_INTERNAL_ERROR	4052	String function internal error.
ERR_SOME_ARRAY_ERROR	4053	Some array error.
ERR_INCORRECT_SERIES_ARRAY_USING	4054	Incorrect series array using.
ERR_CUSTOM_INDICATOR_ERROR	4055	Custom indicator error.
ERR_INCOMPATIBLE_ARRAYS	4056	Arrays are incompatible.
ERR_GLOBAL_VARIABLES_PROCESSING_ERROR	4057	Global variables processing error.
ERR_GLOBAL_VARIABLE_NOT_FOUND	4058	Global variable not found.
ERR_FUNCTION_NOT_ALLOWED_IN_TESTING_MODE	4059	Function is not allowed in testing mode.
ERR_FUNCTION_NOT_CONFIRMED	4060	Function is not confirmed.
ERR_SEND_MAIL_ERROR	4061	Mail sending error.
ERR_STRING_PARAMETER_EXPECTED	4062	String parameter expected.
ERR_INTEGER_PARAMETER_EXPECTED	4063	Integer parameter expected.
ERR_DOUBLE_PARAMETER_EXPECTED	4064	Double parameter expected.
ERR_ARRAY_AS_PARAMETER_EXPECTED	4065	Array as parameter expected.
ERR_HISTORY_WILL_UPDATED	4066	Requested history data in updating state.
ERR_TRADE_ERROR	4067	Some error in trade operation execution.
ERR_END_OF_FILE	4099	End of a file.
ERR_SOME_FILE_ERROR	4100	Some file error.
ERR_WRONG_FILE_NAME	4101	Wrong file name.
ERR_TOO_MANY_OPENED_FILES	4102	Too many opened files.
ERR_CANNOT_OPEN_FILE	4103	Cannot open file.
ERR_INCOMPATIBLE_ACCESS_TO_FILE	4104	Incompatible access to a file.
ERR_NO_ORDER_SELECTED	4105	No order selected.
ERR_UNKNOWN_SYMBOL	4106	Unknown symbol.
ERR_INVALID_PRICE_PARAM	4107	Invalid price.
ERR_INVALID_TICKET	4108	Invalid ticket.
ERR_TRADE_NOT_ALLOWED	4109	Trade is not allowed.
ERR_LONGS_NOT_ALLOWED	4110	Longs are not allowed.
ERR_SHORTS_NOT_ALLOWED	4111	Shorts are not allowed.
ERR_OBJECT_ALREADY_EXISTS	4200	Object already exists.
ERR_UNKNOWN_OBJECT_PROPERTY	4201	Unknown object property.
ERR_OBJECT_DOES_NOT_EXIST	4202	Object does not exist.
ERR_UNKNOWN_OBJECT_TYPE	4203	Unknown object type.
ERR_NO_OBJECT_NAME	4204	No object name.
ERR_OBJECT_COORDINATES_ERROR	4205	Object coordinates error.
ERR_NO_SPECIFIED_SUBWINDOW	4206	No specified subwindow.
ERR_SOME_OBJECT_ERROR	4207	Some error in object operation.

*/

// Code d'erreur MetaTrader et MQL
const (
	ERR_NO_ERROR                              = 0
	ERR_NO_RESULT                             = 1
	ERR_COMMON_ERROR                          = 2
	ERR_INVALID_TRADE_PARAMETERS              = 3
	ERR_SERVER_BUSY                           = 4
	ERR_OLD_VERSION                           = 5
	ERR_NO_CONNECTION                         = 6
	ERR_NOT_ENOUGH_RIGHTS                     = 7
	ERR_TOO_FREQUENT_REQUESTS                 = 8
	ERR_MALFUNCTIONAL_TRADE                   = 9
	ERR_ACCOUNT_DISABLED                      = 64
	ERR_INVALID_ACCOUNT                       = 65
	ERR_TRADE_TIMEOUT                         = 128
	ERR_INVALID_PRICE                         = 129
	ERR_INVALID_STOPS                         = 130
	ERR_INVALID_TRADE_VOLUME                  = 131
	ERR_MARKET_CLOSED                         = 132
	ERR_TRADE_DISABLED                        = 133
	ERR_NOT_ENOUGH_MONEY                      = 134
	ERR_PRICE_CHANGED                         = 135
	ERR_OFF_QUOTES                            = 136
	ERR_BROKER_BUSY                           = 137
	ERR_REQUOTE                               = 138
	ERR_ORDER_LOCKED                          = 139
	ERR_LONG_POSITIONS_ONLY                   = 140
	ERR_TOO_MANY_REQUESTS                     = 141
	ERR_TRADE_MODIFY_DENIED                   = 145
	ERR_TRADE_CONTEXT_BUSY                    = 146
	ERR_TRADE_EXPIRATION_DENIED               = 147
	ERR_TRADE_TOO_MANY_ORDERS                 = 148
	ERR_NO_MQLERROR                           = 4000
	ERR_WRONG_FUNCTION_POINTER                = 4001
	ERR_ARRAY_INDEX_OUT_OF_RANGE              = 4002
	ERR_NO_MEMORY_FOR_FUNCTION_CALL_STACK     = 4003
	ERR_RECURSIVE_STACK_OVERFLOW              = 4004
	ERR_NOT_ENOUGH_STACK_FOR_PARAMETER        = 4005
	ERR_NO_MEMORY_FOR_PARAMETER_STRING        = 4006
	ERR_NO_MEMORY_FOR_TEMP_STRING             = 4007
	ERR_NOT_INITIALIZED_STRING                = 4008
	ERR_NOT_INITIALIZED_ARRAYSTRING           = 4009
	ERR_NO_MEMORY_FOR_ARRAYSTRING             = 4010
	ERR_TOO_LONG_STRING                       = 4011
	ERR_REMAINDER_FROM_ZERO_DIVIDE            = 4012
	ERR_ZERO_DIVIDE                           = 4013
	ERR_UNKNOWN_COMMAND                       = 4014
	ERR_WRONG_JUMP                            = 4015
	ERR_NOT_INITIALIZED_ARRAY                 = 4016
	ERR_DLL_CALLS_NOT_ALLOWED                 = 4017
	ERR_CANNOT_LOAD_LIBRARY                   = 4018
	ERR_CANNOT_CALL_FUNCTION                  = 4019
	ERR_EXTERNAL_EXPERT_CALLS_NOT_ALLOWED     = 4020
	ERR_NOT_ENOUGH_MEMORY_FOR_RETURNED_STRING = 4021
	ERR_SYSTEM_BUSY                           = 4022
	ERR_INVALID_FUNCTION_PARAMETERS_COUNT     = 4050
	ERR_INVALID_FUNCTION_PARAMETER_VALUE      = 4051
	ERR_STRING_FUNCTION_INTERNAL_ERROR        = 4052
	ERR_SOME_ARRAY_ERROR                      = 4053
	ERR_INCORRECT_SERIES_ARRAY_USING          = 4054
	ERR_CUSTOM_INDICATOR_ERROR                = 4055
	ERR_INCOMPATIBLE_ARRAYS                   = 4056
	ERR_GLOBAL_VARIABLES_PROCESSING_ERROR     = 4057
	ERR_GLOBAL_VARIABLE_NOT_FOUND             = 4058
	ERR_FUNCTION_NOT_ALLOWED_IN_TESTING_MODE  = 4059
	ERR_FUNCTION_NOT_CONFIRMED                = 4060
	ERR_SEND_MAIL_ERROR                       = 4061
	ERR_STRING_PARAMETER_EXPECTED             = 4062
	ERR_INTEGER_PARAMETER_EXPECTED            = 4063
	ERR_DOUBLE_PARAMETER_EXPECTED             = 4064
	ERR_ARRAY_AS_PARAMETER_EXPECTED           = 4065
	ERR_HISTORY_WILL_UPDATED                  = 4066
	ERR_TRADE_ERROR                           = 4067
	ERR_END_OF_FILE                           = 4099
	ERR_SOME_FILE_ERROR                       = 4100
	ERR_WRONG_FILE_NAME                       = 4101
	ERR_TOO_MANY_OPENED_FILES                 = 4102
	ERR_CANNOT_OPEN_FILE                      = 4103
	ERR_INCOMPATIBLE_ACCESS_TO_FILE           = 4104
	ERR_NO_ORDER_SELECTED                     = 4105
	ERR_UNKNOWN_SYMBOL                        = 4106
	ERR_INVALID_PRICE_PARAM                   = 4107
	ERR_INVALID_TICKET                        = 4108
	ERR_TRADE_NOT_ALLOWED                     = 4109
	ERR_LONGS_NOT_ALLOWED                     = 4110
	ERR_SHORTS_NOT_ALLOWED                    = 4111
	ERR_OBJECT_ALREADY_EXISTS                 = 4200
	ERR_UNKNOWN_OBJECT_PROPERTY               = 4201
	ERR_OBJECT_DOES_NOT_EXIST                 = 4202
	ERR_UNKNOWN_OBJECT_TYPE                   = 4203
	ERR_NO_OBJECT_NAME                        = 4204
	ERR_OBJECT_COORDINATES_ERROR              = 4205
	ERR_NO_SPECIFIED_SUBWINDOW                = 4206
	ERR_SOME_OBJECT_ERROR                     = 4207
)

// ErrorType est un type personnalisé pour représenter le type d'erreur.
type ErrorType int

const (
	Success ErrorType = iota
	Retry
	NoRetry
)

// TradeError représente une erreur personnalisée avec un message et le type.
type TradeError struct {
	Code        int
	Description string
	Type        ErrorType
}

func (e *TradeError) Error() string {
	return fmt.Sprintf("Error code: %d, Description: %s", e.Code, e.Description)
}

// Map des erreurs et leur description
var errorMap = map[int]string{
	ERR_NO_ERROR:                              "No error",
	ERR_NO_RESULT:                             "No result",
	ERR_COMMON_ERROR:                          "Common error",
	ERR_INVALID_TRADE_PARAMETERS:              "Invalid trade parameters",
	ERR_SERVER_BUSY:                           "Server is busy",
	ERR_TOO_FREQUENT_REQUESTS:                 "Too frequent requests",
	ERR_PRICE_CHANGED:                         "Price changed",
	ERR_REQUOTE:                               "Requote",
	ERR_NO_MQLERROR:                           "No MQL error",
	ERR_SYSTEM_BUSY:                           "System busy",
	ERR_TRADE_TIMEOUT:                         "Trade timeout",
	ERR_TRADE_CONTEXT_BUSY:                    "Trade context busy",
	ERR_BROKER_BUSY:                           "Broker is busy",
	ERR_TRADE_EXPIRATION_DENIED:               "Expirations are denied by broker",
	ERR_TRADE_TOO_MANY_ORDERS:                 "The amount of opened and pending orders has reached the limit set by a broker",
	ERR_CANNOT_LOAD_LIBRARY:                   "Cannot load library",
	ERR_CANNOT_CALL_FUNCTION:                  "Cannot call function",
	ERR_SEND_MAIL_ERROR:                       "Mail sending error",
	ERR_STRING_PARAMETER_EXPECTED:             "String parameter expected",
	ERR_INTEGER_PARAMETER_EXPECTED:            "Integer parameter expected",
	ERR_DOUBLE_PARAMETER_EXPECTED:             "Double parameter expected",
	ERR_ARRAY_AS_PARAMETER_EXPECTED:           "Array as parameter expected",
	ERR_HISTORY_WILL_UPDATED:                  "Requested history data in updating state",
	ERR_TRADE_ERROR:                           "Some error in trade operation execution",
	ERR_END_OF_FILE:                           "End of a file",
	ERR_SOME_FILE_ERROR:                       "Some file error",
	ERR_WRONG_FILE_NAME:                       "Wrong file name",
	ERR_TOO_MANY_OPENED_FILES:                 "Too many opened files",
	ERR_CANNOT_OPEN_FILE:                      "Cannot open file",
	ERR_INCOMPATIBLE_ACCESS_TO_FILE:           "Incompatible access to a file",
	ERR_NO_ORDER_SELECTED:                     "No order selected",
	ERR_UNKNOWN_SYMBOL:                        "Unknown symbol",
	ERR_INVALID_PRICE_PARAM:                   "Invalid price",
	ERR_INVALID_TICKET:                        "Invalid ticket",
	ERR_TRADE_NOT_ALLOWED:                     "Trade is not allowed",
	ERR_LONGS_NOT_ALLOWED:                     "Longs are not allowed",
	ERR_SHORTS_NOT_ALLOWED:                    "Shorts are not allowed",
	ERR_OBJECT_ALREADY_EXISTS:                 "Object already exists",
	ERR_UNKNOWN_OBJECT_PROPERTY:               "Unknown object property",
	ERR_OBJECT_DOES_NOT_EXIST:                 "Object does not exist",
	ERR_UNKNOWN_OBJECT_TYPE:                   "Unknown object type",
	ERR_NO_OBJECT_NAME:                        "No object name",
	ERR_OBJECT_COORDINATES_ERROR:              "Object coordinates error",
	ERR_NO_SPECIFIED_SUBWINDOW:                "No specified subwindow",
	ERR_SOME_OBJECT_ERROR:                     "Some error in object operation",
	ERR_WRONG_FUNCTION_POINTER:                "Wrong function pointer",
	ERR_ARRAY_INDEX_OUT_OF_RANGE:              "Array index is out of range",
	ERR_NO_MEMORY_FOR_FUNCTION_CALL_STACK:     "No memory for function call stack",
	ERR_RECURSIVE_STACK_OVERFLOW:              "Recursive stack overflow",
	ERR_NOT_ENOUGH_STACK_FOR_PARAMETER:        "Not enough stack for parameter",
	ERR_NO_MEMORY_FOR_PARAMETER_STRING:        "No memory for parameter string",
	ERR_NO_MEMORY_FOR_TEMP_STRING:             "No memory for temp string",
	ERR_NOT_INITIALIZED_STRING:                "Not initialized string",
	ERR_NOT_INITIALIZED_ARRAYSTRING:           "Not initialized string in an array",
	ERR_NO_MEMORY_FOR_ARRAYSTRING:             "No memory for an array string",
	ERR_TOO_LONG_STRING:                       "Too long string",
	ERR_REMAINDER_FROM_ZERO_DIVIDE:            "Remainder from zero divide",
	ERR_ZERO_DIVIDE:                           "Zero divide",
	ERR_UNKNOWN_COMMAND:                       "Unknown command",
	ERR_WRONG_JUMP:                            "Wrong jump",
	ERR_NOT_INITIALIZED_ARRAY:                 "Not initialized array",
	ERR_DLL_CALLS_NOT_ALLOWED:                 "DLL calls are not allowed",
	ERR_EXTERNAL_EXPERT_CALLS_NOT_ALLOWED:     "EA function calls are not allowed",
	ERR_NOT_ENOUGH_MEMORY_FOR_RETURNED_STRING: "Not enough memory for a string returned from a function",
	ERR_INVALID_FUNCTION_PARAMETERS_COUNT:     "Invalid function parameters count",
	ERR_INVALID_FUNCTION_PARAMETER_VALUE:      "Invalid function parameter value",
	ERR_STRING_FUNCTION_INTERNAL_ERROR:        "String function internal error",
	ERR_SOME_ARRAY_ERROR:                      "Some array error",
	ERR_INCORRECT_SERIES_ARRAY_USING:          "Incorrect series array using",
	ERR_CUSTOM_INDICATOR_ERROR:                "Custom indicator error",
	ERR_INCOMPATIBLE_ARRAYS:                   "Arrays are incompatible",
	ERR_GLOBAL_VARIABLES_PROCESSING_ERROR:     "Global variables processing error",
	ERR_GLOBAL_VARIABLE_NOT_FOUND:             "Global variable not found",
	ERR_INVALID_STOPS:                         "Invalid stops",
	ERR_TRADE_MODIFY_DENIED:                   "Modification denied because an order is too close to market",
	ERR_TRADE_DISABLED:                        "Trade is disabled",
	ERR_NOT_ENOUGH_MONEY:                      "Not enough money",
	ERR_OFF_QUOTES:                            "Off quotes",
	ERR_ORDER_LOCKED:                          "Order is locked",
	ERR_LONG_POSITIONS_ONLY:                   "Long positions only allowed",
	ERR_TOO_MANY_REQUESTS:                     "Too many requests",
	ERR_OLD_VERSION:                           "Old version of the client terminal",
	ERR_INVALID_PRICE:                         "Invalid price",
	ERR_INVALID_TRADE_VOLUME:                  "Invalid trade volume",
	ERR_MARKET_CLOSED:                         "Market is closed",
	ERR_NO_CONNECTION:                         "No connection with trade server",
	ERR_NOT_ENOUGH_RIGHTS:                     "Not enough rights",
	ERR_INVALID_ACCOUNT:                       "Invalid account",
	//	ERR_MALFUNCTIONAL_TRADE                   = 9
	//	ERR_ACCOUNT_DISABLED
	ERR_MALFUNCTIONAL_TRADE:                  "Malfunctional trade operation",
	ERR_ACCOUNT_DISABLED:                     "Account disabled",
	ERR_FUNCTION_NOT_ALLOWED_IN_TESTING_MODE: "Function is not allowed in testing mode",
	ERR_FUNCTION_NOT_CONFIRMED:               "Function is not confirmed",
}

// HandleTradeError prend un code d'erreur et renvoie une erreur personnalisée selon le type.
func HandleTradeError(code int) error {
	description, exists := errorMap[code]
	if !exists {
		return errors.New("Unknown error code")
	}

	var errorType ErrorType
	switch code {
	case ERR_NO_ERROR, ERR_NO_RESULT, ERR_NO_MQLERROR:
		errorType = Success
	case ERR_SERVER_BUSY, ERR_TOO_FREQUENT_REQUESTS, ERR_PRICE_CHANGED, ERR_REQUOTE, ERR_SYSTEM_BUSY, ERR_TRADE_TIMEOUT, ERR_TRADE_CONTEXT_BUSY:
		// Donne la priorité au retry pour ces erreurs récupérables
		errorType = Retry
	default:
		errorType = NoRetry
	}

	return &TradeError{
		Code:        code,
		Description: description,
		Type:        errorType,
	}
}

func main() {
	// Exemple d'utilisation avec des erreurs spécifiques
	codesToTest := []int{
		ERR_NO_ERROR, ERR_SERVER_BUSY, ERR_INVALID_TRADE_PARAMETERS, ERR_SYSTEM_BUSY, ERR_NO_MQLERROR, ERR_PRICE_CHANGED,
	}

	for _, code := range codesToTest {
		err := HandleTradeError(code)
		if tradeErr, ok := err.(*TradeError); ok {
			fmt.Printf("Error: %s, Type: %v\n", tradeErr.Description, tradeErr.Type)
		}
	}
}
