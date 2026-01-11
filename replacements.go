package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func runReplacementsAndReturnModifiedBody(body []byte, r *http.Request) []byte {
	typedBody := QueryResponse{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&typedBody)
	if err != nil {
		log.Println("Error when decoding JSON (", string(body), ") will return unhandled:", err)
		return body
	} else {
		if typedBody.QueryResult == nil || typedBody.QueryId == "" {
			log.Println("Unexpected query response, will return unhandled.")
			return body
		}

		if typedBody.QueryResult.Intent.Name == "chatgpt_speak" && conf.ChatGptSpeakServer != "" {
			speakResponse := makeChatGptSpeakRequest(typedBody.QueryResult.QueryText, typedBody.LanguageCode, typedBody.QueryResult.BehaviorParas.Txt, r)
			if speakResponse.Url != "" && speakResponse.Txt != "" {
				log.Println("Successfully replaced chatgpt_speak response for request.")
				typedBody.QueryResult.BehaviorParas.Url = speakResponse.Url
				typedBody.QueryResult.BehaviorParas.Txt = speakResponse.Txt
			} else {
				log.Println("Failed to get valid response from ChatGptSpeakServer, keeping original response.")
			}
		}
		modifiedBody, err := json.Marshal(typedBody)
		if err != nil {
			log.Println("Error when marshaling modified JSON, will return unhandled:", err)
			return body
		}
		return modifiedBody
	}
}

func makeChatGptSpeakRequest(queryText string, languageCode string, fallbackResponse string, r *http.Request) BehaviorParas {

	type EmoAutherizationHeaders struct {
		Authorization string `json:"Authorization,omitempty"`
		Secret        string `json:"Secret,omitempty"`
	}
	type ChatGptSpeakRequest struct {
		QueryText            string                  `json:"queryText"`
		LanguageCode         string                  `json:"languageCode"`
		FallbackResponse     string                  `json:"fallbackResponse,omitempty"`
		AutherizationHeaders EmoAutherizationHeaders `json:"authorizationHeaders,omitempty"`
	}
	type ChatGptSpeakResponse struct {
		StatusCode        int    `json:"statusCode"`
		StatusMessage     string `json:"statusMessage"`
		ResponseText      string `json:"responseText"`
		ResponseSpeechUrl string `json:"responseSpeechUrl"`
	}

	chatGptRequestData := ChatGptSpeakRequest{
		QueryText:        queryText,
		LanguageCode:     languageCode,
		FallbackResponse: fallbackResponse,
	}

	authorizationHeader, authHeaderexists := r.Header["Authorization"]
	secretVal, secretExists := r.Header["Secret"]
	if authHeaderexists && secretExists {
		authHeaders := EmoAutherizationHeaders{
			Authorization: authorizationHeader[0],
			Secret:        secretVal[0],
		}
		chatGptRequestData.AutherizationHeaders = authHeaders
	}

	chatGptRequestBody, _ := json.Marshal(chatGptRequestData)
	chatGptRequest, _ := http.NewRequest("POST", conf.ChatGptSpeakServer+"/speak", bytes.NewBuffer(chatGptRequestBody))
	chatGptRequest.Header.Add("Content-Type", "application/json")

	chatGptClient := &http.Client{}
	chatGptResponse, err := chatGptClient.Do(chatGptRequest)
	if err != nil {
		log.Fatalf("An Error Occured while calling ChatGptSpeakServer %v", err)
	}
	defer chatGptResponse.Body.Close()

	chatGptResponseBody, _ := io.ReadAll(chatGptResponse.Body)

	var chatGptTypedResponse ChatGptSpeakResponse
	if err := json.Unmarshal([]byte(chatGptResponseBody), &chatGptTypedResponse); err != nil {
		log.Printf("Error unmarshaling ChatGptSpeakServer response: %v\n", err)
		return BehaviorParas{}
	}

	if chatGptTypedResponse.StatusCode != 200 {
		log.Printf("ChatGptSpeakServer returned non-200 status: %d, message: %s\n", chatGptTypedResponse.StatusCode, chatGptTypedResponse.StatusMessage)
		return BehaviorParas{}
	}
	if chatGptTypedResponse.ResponseText == "" {
		log.Println("ChatGptSpeakServer returned empty response text")
		return BehaviorParas{}
	}
	if chatGptTypedResponse.ResponseSpeechUrl == "" {
		log.Println("ChatGptSpeakServer returned empty response speech URL")
		return BehaviorParas{}
	}

	behaviorParasResponse := BehaviorParas{
		Txt: chatGptTypedResponse.ResponseText,
		Url: chatGptTypedResponse.ResponseSpeechUrl,
	}

	return behaviorParasResponse
}
