import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class VirtualThreadExecutor {
    public static void main(String[] args) {
        Instant start = Instant.now();

        // 작업 하나당 가상 스레드 하나. try-with-resources가 모든 작업이 끝날 때까지 기다림
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            IntStream.range(0, 10_000).forEach(i ->
                    executor.submit(() -> {
                        Thread.sleep(Duration.ofSeconds(1)); // 블
                        return i;
                    }));
        }

        System.out.println("elapsed: " + Duration.between(start, Instant.now()).toMillis() + "ms");
        // 약 1초 걸림. 플랫폼 스레드 풀(예: 200개)로 하면 약 50초
    }
}