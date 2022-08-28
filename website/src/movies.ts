export interface Movie {

}

class Movies {
    knownMovies: Movie[]

    constructor() {
        this.knownMovies = []
    }
}

export const movies = new Movies()