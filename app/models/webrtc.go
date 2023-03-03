package models

type OfferBodyST struct {
	OfferBase64 string `json:"offer_base64"validate:"required"`
}

type AnswerST struct {
	AnswerBase64 string `json:"answer_base64"validate:"required"`
}
