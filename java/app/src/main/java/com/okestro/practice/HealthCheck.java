package com.okestro.practice;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public class HealthCheck {
    private static final String URL = "http://localhost:8080";
    private static final String NAME = "Heonjinjeong";
    private static final int MAX_RETRIES = 3;
    private static final Duration TIMEOUT = Duration.ofSeconds(10);
    private static final long INTERVAL_MS = 60_000;

    public static void main(String[] args) throws InterruptedException {
        HttpClient client = HttpClient.newBuilder()
                .connectTimeout(TIMEOUT)
                .build();

        HttpRequest request = HttpRequest.newBuilder(URI.create(URL))
                .timeout(TIMEOUT)  // 응답 대기 10초
                .header("Content-Type", "text/plain; charset=UTF-8")
                .POST(HttpRequest.BodyPublishers.ofString(NAME))
                .build();

        while (true) {
            sendWithRetry(client, request);
            Thread.sleep(INTERVAL_MS);  // 1분 주기
        }
    }

    private static void sendWithRetry(HttpClient client, HttpRequest request)
            throws InterruptedException {
        // 최초 시도 1회 + 재시도 3회
        for (int attempt = 0; attempt <= MAX_RETRIES; attempt++) {
            try {
                HttpResponse<String> response =
                        client.send(request, HttpResponse.BodyHandlers.ofString());
                System.out.println("Sent OK, status=" + response.statusCode());
                return;
            } catch (IOException e) {  // HttpTimeoutException, ConnectException 포함
                System.out.println("Attempt " + (attempt + 1) + " failed: " + e);
            }
        }
        System.out.println("All retries failed. Waiting for next cycle.");
    }
}