package llamacpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	CompletionDataPrefix = "data: "
)

var (
	ErrorNon200Response = errors.New("Non-200 response from llama.cpp")
)

type LlamaCppClient struct {
	HttpClient            *http.Client
	LlamaCppConfiguration *LlamaCppConfiguration
}

func (self *LlamaCppClient) GetHealth(
	ctx context.Context,
	responseChannel chan<- LlamaCppHealthStatus,
) {
	request, err := http.NewRequestWithContext(
		ctx,
		"GET",
		self.LlamaCppConfiguration.HttpAddress.BuildUrlWithPath("health").String(),
		nil,
	)

	if err != nil {
		responseChannel <- LlamaCppHealthStatus{
			Error:        err,
			ErrorMessage: err.Error(),
			Status:       Error,
		}

		return
	}

	response, err := self.HttpClient.Do(request)

	if err != nil {
		responseChannel <- LlamaCppHealthStatus{
			Error:        err,
			ErrorMessage: err.Error(),
			Status:       Error,
		}

		return
	}

	defer response.Body.Close()

	if http.StatusOK != response.StatusCode {
		responseChannel <- LlamaCppHealthStatus{
			Error:        ErrorNon200Response,
			ErrorMessage: ErrorNon200Response.Error(),
			Status:       Error,
		}

		return
	}

	var llamaCppHealthStatus LlamaCppHealthStatus

	err = json.NewDecoder(response.Body).Decode(&llamaCppHealthStatus)

	if err != nil {
		responseChannel <- LlamaCppHealthStatus{
			Error:        err,
			ErrorMessage: err.Error(),
			Status:       Error,
		}

		return
	}

	responseChannel <- llamaCppHealthStatus
}

func (self *LlamaCppClient) GenerateCompletion(
	ctx context.Context,
	responseChannel chan LlamaCppCompletionToken,
	llamaCppCompletionRequest LlamaCppCompletionRequest,
) {
	defer close(responseChannel)

	body, err := json.Marshal(llamaCppCompletionRequest)

	if err != nil {
		responseChannel <- LlamaCppCompletionToken{
			Error: err,
		}

		return
	}

	request, err := http.NewRequestWithContext(
		ctx,
		"POST",
		self.LlamaCppConfiguration.HttpAddress.BuildUrlWithPath("completion").String(),
		bytes.NewBuffer(body),
	)

	if err != nil {
		responseChannel <- LlamaCppCompletionToken{
			Error: err,
		}

		return
	}

	response, err := self.HttpClient.Do(request)

	if err != nil {
		responseChannel <- LlamaCppCompletionToken{
			Error: err,
		}

		return
	}

	defer response.Body.Close()

	if http.StatusOK != response.StatusCode {
		responseChannel <- LlamaCppCompletionToken{
			Error: ErrorNon200Response,
		}

		return
	}

	reader := bufio.NewReader(response.Body)

	for {
		line, err := reader.ReadBytes('\n')

		if err != nil && err != io.EOF {
			responseChannel <- LlamaCppCompletionToken{
				Error: err,
			}

			return
		}

		var llamaCppCompletionToken LlamaCppCompletionToken

		trimmedLine := bytes.TrimPrefix(line, []byte(CompletionDataPrefix))

		if len(trimmedLine) < 2 {
			continue
		}

		err = json.Unmarshal(trimmedLine, &llamaCppCompletionToken)

		if err != nil {
			responseChannel <- LlamaCppCompletionToken{
				Error: err,
			}

			return
		}

		responseChannel <- llamaCppCompletionToken

		if llamaCppCompletionToken.IsLast {
			return
		}
	}
}
