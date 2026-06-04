package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"LavenderMessenger/gen"
)

func main() {
	serverAddr := flag.String("server", "localhost:50052", "gRPC server address")
	userID := flag.String("user", "test-user-001", "User ID")
	message := flag.String("msg", "Привет! Как дела?", "Message to send")
	imagePath := flag.String("image", "", "Path to image file (optional)")
	modelHint := flag.String("model", "", "Model hint (empty=default, local/=Hermes)")
	flag.Parse()

	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := gen.NewChatServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &gen.PipelineRequest{
		UserId:    *userID,
		Message:   *message,
		ModelHint: *modelHint,
	}

	// Если указана картинка — загружаем
	if *imagePath != "" {
		imgData, err := os.ReadFile(*imagePath)
		if err != nil {
			log.Fatalf("Failed to read image: %v", err)
		}
		req.Images = append(req.Images, imgData)
		fmt.Printf("📎 Attached image: %s (%d bytes)\n", *imagePath, len(imgData))
	}

	fmt.Printf("📤 Sending to %s: %q (model=%q)\n", *serverAddr, *message, *modelHint)

	stream, err := client.ChatWithPipeline(ctx, req)
	if err != nil {
		log.Fatalf("ChatWithPipeline error: %v", err)
	}

	fmt.Println("📥 Response:")
	var fullResponse string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Recv error: %v", err)
			break
		}

		if resp.Token != "" {
			fmt.Print(resp.Token)
			fullResponse += resp.Token
		}

		if resp.Error != "" {
			fmt.Printf("\n❌ Error: %s\n", resp.Error)
			break
		}

		if resp.Finished {
			fmt.Println("\n✅ Done")
			break
		}
	}

	if fullResponse != "" {
		fmt.Printf("\n📝 Full response length: %d chars\n", len(fullResponse))
	}
}
