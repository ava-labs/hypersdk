

export default function Errors({ errors }: { errors: string[] }) {
    return (
        <div className="flex items-center justify-center min-h-screen">
            <div className="bg-white p-6 rounded-lg shadow-md max-w-md w-full ">
                <div className="text-lg font-bold  mb-4">Errors:</div>
                <ol className="text-lg list-disc pl-5 mb-5">
                    {errors.map((error, index) => (
                        <li key={index} className="mb-2">{error}</li>
                    ))}
                </ol>
                <button className="bg-blue-500 hover:bg-blue-700 text-white px-4 py-2 rounded-md" onClick={() =>
                    window.location.reload()
                }>
                    Start over
                </button>
            </div>
        </div>
    )
}