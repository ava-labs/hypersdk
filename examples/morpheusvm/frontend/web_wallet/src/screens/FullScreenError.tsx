

export default function FullScreenError({ errors }: { errors: string[] }) {
    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="border border-black  p-6 rounded-lg max-w-md w-full ">
                <div className="text-lg font-bold  mb-4">Errors:</div>
                <ol className="text-lg list-disc pl-5 mb-5">
                    {errors.map((error, index) => (
                        <li key={index} className="mb-2">{error}</li>
                    ))}
                </ol>
                <button className="px-4 py-2 bg-black text-white font-bold rounded hover:bg-gray-800 transition-colors duration-200 transform hover:scale-105"
                    onClick={() =>
                        window.location.reload()
                    }>
                    Start over
                </button>
            </div>
        </div>
    )
}