#pragma once

#include <atomic>

namespace protocomm::detail {

// Intrusive lock-free multi-producer / single-consumer queue (Vyukov's algorithm).
template <class Node>
class MpscQueue {
public:
    MpscQueue() : head_(&stub_), tail_(&stub_) {
        stub_.mpsc_next.store(nullptr, std::memory_order_relaxed);
    }

    MpscQueue(const MpscQueue&) = delete;
    MpscQueue& operator=(const MpscQueue&) = delete;

    // Any thread.
    void push(Node* node) {
        node->mpsc_next.store(nullptr, std::memory_order_relaxed);
        Node* prev = head_.exchange(node, std::memory_order_acq_rel);
        prev->mpsc_next.store(node, std::memory_order_release);
    }

    // Consumer thread only. Returns nullptr if empty (or transiently inconsistent).
    Node* pop() {
        Node* tail = tail_;
        Node* next = tail->mpsc_next.load(std::memory_order_acquire);

        if (tail == &stub_) {
            if (next == nullptr) {
                return nullptr;
            }
            tail_ = next;
            tail = next;
            next = next->mpsc_next.load(std::memory_order_acquire);
        }

        if (next != nullptr) {
            tail_ = next;
            return tail;
        }

        // tail is the last node; if a producer hasn't linked a successor yet, bail.
        if (tail != head_.load(std::memory_order_acquire)) {
            return nullptr;
        }

        // Re-insert the stub so the queue is never empty, then retry once.
        push(&stub_);
        next = tail->mpsc_next.load(std::memory_order_acquire);
        if (next != nullptr) {
            tail_ = next;
            return tail;
        }
        return nullptr;
    }

private:
    std::atomic<Node*> head_; // producers exchange here
    Node* tail_;              // consumer only
    Node stub_;               // sentinel; never carries a payload
};

}
