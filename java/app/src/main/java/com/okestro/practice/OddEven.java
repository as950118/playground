package com.okestro.practice;
public class OddEven {
    private static final int MAX = 1000;
    private static int n = 1;
    private static boolean stopped = false; // lock으로 보호
    private static final Object lock = new Object();

    static Runnable printer(boolean odd) {
        return () -> {
            synchronized (lock) {
                try {
                    while (!stopped && n <= MAX) {
                        if ((n % 2 == 1) == odd) {
                            System.out.println(Thread.currentThread().getName() + ": " + n++);
                            lock.notifyAll();
                        } else {
                            System.out.println(Thread.currentThread().getName() + "WAIT");
                            lock.wait();
                        }
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt(); // 인터럽트 상태 복원
                    stopped = true;                     // 상대에게 종료 알림
                } finally {
                    lock.notifyAll();                   // 정상/비정상 종료 모두 상대 깨움
                }
            }
        };
    }

    public static void main(String[] args) {
        new Thread(printer(true), "odd").start();
        new Thread(printer(false), "even").start();
    }
}