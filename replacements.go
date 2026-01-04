package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
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

func makeEmoSpeechRequest(text string, languageCode string, r *http.Request) EmoSpeechResponse {
	request, _ := http.NewRequest("GET", "https://"+conf.Livingio_API_Server+"/emo/speech/tts?q="+url.QueryEscape(text)+"&l="+url.QueryEscape(languageCode), nil)

	val, exists := r.Header["Authorization"]
	if exists {
		request.Header.Add("Authorization", val[0])
	}

	val, exists = r.Header["Secret"]
	if exists {
		request.Header.Add("Secret", val[0])
	}

	request.Header.Del("User-Agent")

	httpclient := &http.Client{}
	response, err := httpclient.Do(request)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(response.Body)

	var emoSpeechResponse EmoSpeechResponse
	if err := json.Unmarshal([]byte(body), &emoSpeechResponse); err != nil {
		log.Printf("Error unmarshaling ChatGptSpeakServer response: %v\n", err)
		return EmoSpeechResponse{}
	}

	return emoSpeechResponse
}

func makeChatGptSpeakRequest(queryText string, languageCode string, fallbackResponse string, r *http.Request) BehaviorParas {
	type ChatGptSpeakRequest struct {
		QueryText        string `json:"queryText"`
		LanguageCode     string `json:"languageCode"`
		FallbackResponse string `json:"fallbackResponse,omitempty"`
	}
	type ChatGptSpeakResponse struct {
		ResponseText string `json:"responseText"`
	}

	chatGptRequestData := ChatGptSpeakRequest{
		QueryText:        queryText,
		LanguageCode:     languageCode,
		FallbackResponse: fallbackResponse,
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

	if chatGptTypedResponse.ResponseText == "" {
		log.Println("ChatGptSpeakServer returned empty response text")
		return BehaviorParas{}
	}

	emoSpeechResponse := makeEmoSpeechRequest(chatGptTypedResponse.ResponseText, languageCode, r)
	if emoSpeechResponse.Code != 200 || emoSpeechResponse.Url == "" {
		log.Printf("Error in EmoSpeechResponse: Code %d, Errmessage: %s\n", emoSpeechResponse.Code, emoSpeechResponse.Errmessage)
		return BehaviorParas{}
	}
	behaviorParasResponse := BehaviorParas{
		Txt: chatGptTypedResponse.ResponseText,
		Url: emoSpeechResponse.Url,
	}

	return behaviorParasResponse
}
