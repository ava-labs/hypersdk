

export default function Loading({ text }: { text: string }) {
    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="text-2xl font-bold">{text}</div>
        </div>
    )
}